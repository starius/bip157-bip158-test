//! Adapter binary that exposes Nakamoto through the conformance test API.
//!
//! The adapter keeps Nakamoto behind the same small HTTP/JSON boundary used by
//! the other suite targets. The harness supplies regtest peers and the adapter
//! disables any need for DNS seeds by putting those peers directly into
//! Nakamoto's explicit `connect` list.

use std::{
    collections::{HashMap, HashSet},
    net::{SocketAddr, TcpStream},
    ops::RangeFrom,
    path::PathBuf,
    sync::{mpsc, Arc, Mutex},
    thread,
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use axum::{extract::State, http::StatusCode, routing::post, Json, Router};
use nakamoto_client::{
    handle::Handle as NakamotoHandle, Client, Config, Domain, LoadingHandler, Network,
};
use nakamoto_common::{
    bitcoin::{network::constants::ServiceFlags, Script},
    block::{Block, Height},
};
use serde::{Deserialize, Serialize};
use tokio::net::TcpListener;

type Reactor = nakamoto_net_poll::Reactor<TcpStream>;
type Handle = nakamoto_client::Handle<<Reactor as nakamoto_net::Reactor>::Waker>;
type Shared = Arc<Mutex<AdapterState>>;

/// A configured peer controlled by the conformance harness.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct PeerConfig {
    id: String,
    address: String,
    trusted: bool,
}

/// Request body for configuring one isolated adapter run.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct ConfigureRequest {
    network: String,
    data_dir: String,
    peers: Vec<PeerConfig>,
    required_peers: u32,
    allow_discovery: bool,
}

/// Hash and height pair returned by best-block and block-hash calls.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct BlockRef {
    hash_hex: String,
    height: u32,
}

/// Request body for registering a watched script.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct WatchScriptRequest {
    script_pubkey_hex: String,
    start_height: u32,
}

/// Normalized transaction match reported to the harness.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct TxMatch {
    txid_hex: String,
    block_hash_hex: String,
    height: u32,
    kind: String,
    script_pubkey_hex: String,
    #[serde(default, skip_serializing_if = "is_zero")]
    vout: u32,
    #[serde(default, skip_serializing_if = "is_zero")]
    vin: u32,
}

/// Request body for reading matches.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct GetMatchesRequest {
    script_pubkey_hex: String,
    start_height: u32,
    stop_height: u32,
}

/// Response body for reading matches.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct GetMatchesResponse {
    matches: Vec<TxMatch>,
}

/// Peer state visible through Nakamoto's public handle.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct PeerState {
    id: String,
    address: String,
    connected: bool,
    banned: bool,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    last_error: String,
    #[serde(default)]
    best_height: u32,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    best_hash_hex: String,
}

/// Response body for listing peers.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct ListPeersResponse {
    peers: Vec<PeerState>,
}

/// Minimal liveness response.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct HealthResponse {
    alive: bool,
    status: String,
}

/// One address environment capability reported to the harness.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct EnvironmentCapability {
    id: String,
    supported: bool,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    reason: String,
}

/// Optional capability response for address environments.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct CapabilitiesResponse {
    environments: Vec<EnvironmentCapability>,
}

/// Shared adapter state. Threads only hold the lock while copying or recording
/// data; all blocking Nakamoto calls use a cloned handle outside the lock.
#[derive(Default)]
struct AdapterState {
    config: Option<ConfigureRequest>,
    handle: Option<Handle>,
    watches: HashMap<String, WatchScriptRequest>,
    matches: Vec<TxMatch>,
    seen: HashSet<String>,
    outpoints: HashMap<String, HashSet<String>>,
}

#[tokio::main]
async fn main() {
    let listen = std::env::args()
        .skip_while(|arg| arg != "--listen")
        .nth(1)
        .unwrap_or_else(|| "127.0.0.1:0".to_string());

    let state = Arc::new(Mutex::new(AdapterState::default()));
    let app = Router::new()
        .route("/health", post(health))
        .route("/capabilities", post(capabilities))
        .route("/configure", post(configure))
        .route("/start", post(start))
        .route("/stop", post(stop))
        .route("/watch-script", post(watch_script))
        .route("/best-block", post(best_block))
        .route("/block-hash", post(block_hash))
        .route("/matches", post(matches))
        .route("/list-peers", post(list_peers))
        .with_state(state);

    let listener = TcpListener::bind(&listen).await.expect("listen");
    println!("listening=http://{}", listener.local_addr().expect("addr"));
    axum::serve(listener, app).await.expect("serve");
}

async fn capabilities() -> Json<CapabilitiesResponse> {
    Json(clear_ipv4_capabilities())
}

fn clear_ipv4_capabilities() -> CapabilitiesResponse {
    CapabilitiesResponse {
        environments: vec![
            EnvironmentCapability {
                id: "ipv4".to_string(),
                supported: true,
                reason: String::new(),
            },
            EnvironmentCapability {
                id: "ipv6".to_string(),
                supported: false,
                reason: "adapter has not been validated with IPv6 peer identities".to_string(),
            },
            EnvironmentCapability {
                id: "tor-v3".to_string(),
                supported: false,
                reason: "adapter does not configure Tor proxying".to_string(),
            },
            EnvironmentCapability {
                id: "i2p".to_string(),
                supported: false,
                reason: "adapter does not configure I2P proxying".to_string(),
            },
            EnvironmentCapability {
                id: "cjdns".to_string(),
                supported: false,
                reason: "adapter has not been validated with cjdns".to_string(),
            },
        ],
    }
}

async fn health(State(state): State<Shared>) -> Json<HealthResponse> {
    let guard = state.lock().expect("state");
    let status = if guard.handle.is_some() {
        "started"
    } else if guard.config.is_some() {
        "configured"
    } else {
        "idle"
    };
    Json(HealthResponse {
        alive: true,
        status: status.to_string(),
    })
}

async fn configure(
    State(state): State<Shared>,
    Json(req): Json<ConfigureRequest>,
) -> Result<Json<serde_json::Value>, (StatusCode, String)> {
    if req.network != "regtest" {
        return Err((StatusCode::BAD_REQUEST, "only regtest is supported".into()));
    }
    for peer in &req.peers {
        parse_addr(&peer.address)?;
    }

    let mut guard = state.lock().expect("state");
    stop_locked(&mut guard);
    guard.config = Some(req);
    guard.watches.clear();
    guard.matches.clear();
    guard.seen.clear();
    guard.outpoints.clear();
    Ok(Json(serde_json::json!({"ok": true})))
}

async fn start(
    State(state): State<Shared>,
) -> Result<Json<serde_json::Value>, (StatusCode, String)> {
    let req = {
        let guard = state.lock().expect("state");
        guard
            .config
            .clone()
            .ok_or((StatusCode::CONFLICT, "configure before start".into()))?
    };

    let mut config = Config::new(Network::Regtest);
    config.connect = req
        .peers
        .iter()
        .map(|peer| parse_addr(&peer.address))
        .collect::<Result<Vec<_>, _>>()?;
    config.domains = vec![Domain::IPV4];
    config.listen = vec![([127, 0, 0, 1], 0).into()];
    config.root = data_root(&req);
    config.verify = true;
    config.limits.max_outbound_peers = req.required_peers.max(1) as usize;

    let (ready_tx, ready_rx) = mpsc::channel::<
        Result<(Handle, nakamoto_client::chan::Receiver<(Block, Height)>), String>,
    >();
    thread::spawn(move || {
        let Ok(client) = Client::<Reactor>::new() else {
            let _ = ready_tx.send(Err("create Nakamoto client failed".into()));
            return;
        };
        let mut handle = client.handle();
        handle.set_timeout(Duration::from_secs(5));
        let blocks = handle.blocks();
        let runner = match client.load(config, LoadingHandler::Ignore) {
            Ok(runner) => runner,
            Err(err) => {
                let _ = ready_tx.send(Err(err.to_string()));
                return;
            }
        };
        if ready_tx.send(Ok((handle, blocks))).is_ok() {
            let _ = runner.run();
        }
    });

    let (handle, blocks) = ready_rx
        .recv_timeout(Duration::from_secs(10))
        .map_err(|err| (StatusCode::SERVICE_UNAVAILABLE, err.to_string()))?
        .map_err(|err| (StatusCode::INTERNAL_SERVER_ERROR, err))?;

    let block_state = Arc::clone(&state);
    thread::spawn(move || block_loop(block_state, blocks));

    let mut guard = state.lock().expect("state");
    stop_locked(&mut guard);
    guard.handle = Some(handle);
    Ok(Json(serde_json::json!({"ok": true})))
}

async fn stop(State(state): State<Shared>) -> Json<serde_json::Value> {
    let mut guard = state.lock().expect("state");
    stop_locked(&mut guard);
    Json(serde_json::json!({"ok": true}))
}

async fn watch_script(
    State(state): State<Shared>,
    Json(req): Json<WatchScriptRequest>,
) -> Result<Json<serde_json::Value>, (StatusCode, String)> {
    let script = parse_script(&req.script_pubkey_hex)?;
    let handle = {
        let mut guard = state.lock().expect("state");
        guard
            .outpoints
            .entry(req.script_pubkey_hex.clone())
            .or_default();
        guard
            .watches
            .insert(req.script_pubkey_hex.clone(), req.clone());
        guard
            .handle
            .clone()
            .ok_or((StatusCode::CONFLICT, "adapter not started".into()))?
    };

    let range: RangeFrom<Height> = req.start_height as Height..;
    handle
        .rescan(range, vec![script].into_iter())
        .map_err(|err| (StatusCode::SERVICE_UNAVAILABLE, err.to_string()))?;
    Ok(Json(serde_json::json!({"ok": true})))
}

async fn best_block(State(state): State<Shared>) -> Result<Json<BlockRef>, (StatusCode, String)> {
    let handle = handle(&state)?;
    let (height, header, _) = handle
        .get_tip()
        .map_err(|err| (StatusCode::SERVICE_UNAVAILABLE, err.to_string()))?;
    Ok(Json(BlockRef {
        hash_hex: header.block_hash().to_string(),
        height: checked_height(height)?,
    }))
}

async fn block_hash(
    State(state): State<Shared>,
    Json(req): Json<BlockRef>,
) -> Result<Json<BlockRef>, (StatusCode, String)> {
    let handle = handle(&state)?;
    let header = handle
        .get_block_by_height(req.height as Height)
        .map_err(|err| (StatusCode::SERVICE_UNAVAILABLE, err.to_string()))?
        .ok_or((StatusCode::NOT_FOUND, "height not known".into()))?;
    Ok(Json(BlockRef {
        hash_hex: header.block_hash().to_string(),
        height: req.height,
    }))
}

async fn matches(
    State(state): State<Shared>,
    Json(req): Json<GetMatchesRequest>,
) -> Json<GetMatchesResponse> {
    let guard = state.lock().expect("state");
    let matches = guard
        .matches
        .iter()
        .filter(|m| {
            m.script_pubkey_hex == req.script_pubkey_hex
                && m.height >= req.start_height
                && m.height <= req.stop_height
        })
        .cloned()
        .collect();
    Json(GetMatchesResponse { matches })
}

async fn list_peers(
    State(state): State<Shared>,
) -> Result<Json<ListPeersResponse>, (StatusCode, String)> {
    let (configured, handle) = {
        let guard = state.lock().expect("state");
        (
            guard.config.clone().unwrap_or(ConfigureRequest {
                network: "regtest".into(),
                data_dir: String::new(),
                peers: Vec::new(),
                required_peers: 0,
                allow_discovery: false,
            }),
            guard.handle.clone(),
        )
    };
    let Some(handle) = handle else {
        return Ok(Json(ListPeersResponse { peers: Vec::new() }));
    };

    let peers = handle
        .get_peers(ServiceFlags::NETWORK)
        .unwrap_or_else(|_| Vec::new());
    let tip = handle.get_tip().ok();
    let connected: HashMap<String, _> = peers
        .into_iter()
        .map(|peer| (peer.addr.to_string(), peer))
        .collect();
    let states = configured
        .peers
        .iter()
        .map(|peer| {
            let connected_peer = connected.get(&peer.address);
            let connected = connected_peer.is_some();
            PeerState {
                id: peer.id.clone(),
                address: peer.address.clone(),
                connected,
                banned: false,
                last_error: if connected {
                    String::new()
                } else {
                    "not connected or disconnected".into()
                },
                best_height: connected_peer
                    .map(|p| checked_height(p.height).unwrap_or(u32::MAX))
                    .or_else(|| {
                        tip.map(|(height, _, _)| checked_height(height).unwrap_or(u32::MAX))
                    })
                    .unwrap_or_default(),
                best_hash_hex: tip
                    .as_ref()
                    .map(|(_, header, _)| header.block_hash().to_string())
                    .unwrap_or_default(),
            }
        })
        .collect();
    Ok(Json(ListPeersResponse { peers: states }))
}

fn block_loop(state: Shared, blocks: nakamoto_client::chan::Receiver<(Block, Height)>) {
    while let Ok((block, height)) = blocks.recv() {
        let block_hash = block.block_hash().to_string();
        record_block(
            &state,
            checked_height(height).unwrap_or(u32::MAX),
            block_hash,
            block,
        );
    }
}

fn record_block(state: &Shared, height: u32, block_hash: String, block: Block) {
    let mut guard = state.lock().expect("state");
    let script_hexes: Vec<String> = guard.watches.keys().cloned().collect();
    for tx in block.txdata {
        let txid = tx.txid().to_string();
        for script_hex in &script_hexes {
            let Ok(script) = parse_script(script_hex) else {
                continue;
            };
            for (vout, output) in tx.output.iter().enumerate() {
                if output.script_pubkey != script {
                    continue;
                }
                let outpoint = format!("{txid}:{vout}");
                guard
                    .outpoints
                    .entry(script_hex.clone())
                    .or_default()
                    .insert(outpoint);
                add_match(
                    &mut guard,
                    TxMatch {
                        txid_hex: txid.clone(),
                        block_hash_hex: block_hash.clone(),
                        height,
                        kind: "output".into(),
                        script_pubkey_hex: script_hex.clone(),
                        vout: vout as u32,
                        vin: 0,
                    },
                );
            }
            for (vin, input) in tx.input.iter().enumerate() {
                let outpoint = format!(
                    "{}:{}",
                    input.previous_output.txid, input.previous_output.vout
                );
                if !guard
                    .outpoints
                    .entry(script_hex.clone())
                    .or_default()
                    .contains(&outpoint)
                {
                    continue;
                }
                add_match(
                    &mut guard,
                    TxMatch {
                        txid_hex: txid.clone(),
                        block_hash_hex: block_hash.clone(),
                        height,
                        kind: "spend".into(),
                        script_pubkey_hex: script_hex.clone(),
                        vout: 0,
                        vin: vin as u32,
                    },
                );
            }
        }
    }
}

fn add_match(state: &mut AdapterState, tx_match: TxMatch) {
    let key = format!(
        "{}:{}:{}:{}:{}:{}:{}",
        tx_match.txid_hex,
        tx_match.block_hash_hex,
        tx_match.height,
        tx_match.kind,
        tx_match.script_pubkey_hex,
        tx_match.vout,
        tx_match.vin
    );
    if state.seen.insert(key) {
        state.matches.push(tx_match);
    }
}

fn handle(state: &Shared) -> Result<Handle, (StatusCode, String)> {
    let guard = state.lock().expect("state");
    guard
        .handle
        .clone()
        .ok_or((StatusCode::CONFLICT, "adapter not started".into()))
}

fn stop_locked(state: &mut AdapterState) {
    if let Some(handle) = state.handle.take() {
        let _ = handle.shutdown();
    }
}

fn data_root(req: &ConfigureRequest) -> PathBuf {
    if !req.data_dir.is_empty() {
        return PathBuf::from(&req.data_dir);
    }
    let millis = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis())
        .unwrap_or_default();
    std::env::temp_dir().join(format!("nakamoto-adapter-{millis}"))
}

fn parse_addr(addr: &str) -> Result<SocketAddr, (StatusCode, String)> {
    addr.parse()
        .map_err(|err| (StatusCode::BAD_REQUEST, format!("bad peer address: {err}")))
}

fn parse_script(script_hex: &str) -> Result<Script, (StatusCode, String)> {
    let bytes = hex::decode(script_hex).map_err(|err| {
        (
            StatusCode::BAD_REQUEST,
            format!("invalid script hex: {err}"),
        )
    })?;
    Ok(Script::from(bytes))
}

fn checked_height(height: Height) -> Result<u32, (StatusCode, String)> {
    u32::try_from(height).map_err(|_| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            format!("height {height} does not fit u32"),
        )
    })
}

fn is_zero(value: &u32) -> bool {
    *value == 0
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_script_accepts_hex() {
        let script = parse_script("00141111111111111111111111111111111111111111").expect("script");
        assert_eq!(script.as_bytes().len(), 22);
    }

    #[test]
    fn add_match_deduplicates() {
        let mut state = AdapterState::default();
        let tx_match = TxMatch {
            txid_hex: "tx".into(),
            block_hash_hex: "block".into(),
            height: 1,
            kind: "output".into(),
            script_pubkey_hex: "51".into(),
            vout: 0,
            vin: 0,
        };
        add_match(&mut state, tx_match.clone());
        add_match(&mut state, tx_match);
        assert_eq!(state.matches.len(), 1);
    }

    #[test]
    fn data_root_uses_configured_directory() {
        let req = ConfigureRequest {
            network: "regtest".into(),
            data_dir: "nakamoto-suite".into(),
            peers: Vec::new(),
            required_peers: 1,
            allow_discovery: false,
        };
        assert_eq!(data_root(&req), PathBuf::from("nakamoto-suite"));
    }

    #[test]
    fn capabilities_are_explicit_ipv4_only() {
        let caps = clear_ipv4_capabilities();
        assert_eq!(caps.environments.len(), 5);
        assert!(caps
            .environments
            .iter()
            .any(|cap| cap.id == "ipv4" && cap.supported));
        assert!(caps
            .environments
            .iter()
            .filter(|cap| cap.id != "ipv4")
            .all(|cap| !cap.supported));
    }
}
