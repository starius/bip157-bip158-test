//! Adapter binary that exposes Kyoto through the conformance test API.

use std::{
    collections::{HashMap, HashSet},
    net::SocketAddr,
    path::PathBuf,
    sync::Arc,
    time::Duration,
};

use axum::{extract::State, http::StatusCode, routing::post, Json, Router};
use bip157::{Builder, Client, Event, Network, Node, Requester, ScriptBuf, TrustedPeer};
use serde::{Deserialize, Serialize};
use tokio::{net::TcpListener, sync::Mutex, task::JoinHandle};

/// A configured peer controlled by the conformance harness.
#[derive(Clone, Debug, Deserialize, Serialize)]
struct PeerConfig {
    id: String,
    address: String,
    trusted: bool,
}

/// Request body for configuring one isolated adapter run.
#[derive(Clone, Debug, Default, Deserialize, Serialize)]
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

/// Peer state as visible through Kyoto's public API.
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

/// Shared adapter state guarded by an async mutex.
#[derive(Default)]
struct AdapterState {
    config: ConfigureRequest,
    node: Option<Node>,
    client: Option<Client>,
    requester: Option<Requester>,
    node_task: Option<JoinHandle<()>>,
    event_task: Option<JoinHandle<()>>,
    watches: HashMap<String, WatchScriptRequest>,
    matches: Vec<TxMatch>,
    seen: HashSet<String>,
    outpoints: HashMap<String, HashSet<String>>,
}

type Shared = Arc<Mutex<AdapterState>>;

/// Regtest genesis hash used when Kyoto's requester starts after genesis.
const REGTEST_GENESIS_HASH: &str =
    "0f9188f13cb7b2c71f2a335e3a4fc328bf5beb436012afca590b1a11466e2206";

#[tokio::main]
async fn main() {
    let listen = std::env::args()
        .skip_while(|arg| arg != "--listen")
        .nth(1)
        .unwrap_or_else(|| "127.0.0.1:0".to_string());

    let state = Arc::new(Mutex::new(AdapterState::default()));
    let app = Router::new()
        .route("/health", post(health))
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

async fn health(State(state): State<Shared>) -> Json<HealthResponse> {
    let guard = state.lock().await;
    let status = if guard.requester.is_some() {
        "started"
    } else if guard.node.is_some() {
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

    let mut builder = Builder::new(Network::Regtest)
        .required_peers(req.required_peers.max(1).min(15) as u8)
        .response_timeout(Duration::from_secs(5));
    if !req.allow_discovery {
        builder = builder.whitelist_only();
    }
    if !req.data_dir.is_empty() {
        builder = builder.data_dir(PathBuf::from(&req.data_dir));
    }
    for peer in &req.peers {
        let addr: SocketAddr = peer
            .address
            .parse()
            .map_err(|err| (StatusCode::BAD_REQUEST, format!("bad peer address: {err}")))?;
        builder = builder.add_peer(TrustedPeer::from_socket_addr(addr));
    }

    let (node, client) = builder.build();
    let mut guard = state.lock().await;
    stop_locked(&mut guard);
    guard.config = req;
    guard.node = Some(node);
    guard.client = Some(client);
    guard.watches.clear();
    guard.matches.clear();
    guard.seen.clear();
    guard.outpoints.clear();
    Ok(Json(serde_json::json!({"ok": true})))
}

async fn start(
    State(state): State<Shared>,
) -> Result<Json<serde_json::Value>, (StatusCode, String)> {
    let mut guard = state.lock().await;
    let node = guard
        .node
        .take()
        .ok_or((StatusCode::CONFLICT, "configure before start".into()))?;
    let client = guard
        .client
        .take()
        .ok_or((StatusCode::CONFLICT, "missing client".into()))?;
    let Client {
        requester,
        info_rx: _,
        warn_rx: _,
        event_rx,
    } = client;

    let node_task = tokio::spawn(async move {
        let _ = node.run().await;
    });
    let event_state = Arc::clone(&state);
    let event_requester = requester.clone();
    let event_task = tokio::spawn(async move {
        event_loop(event_state, event_requester, event_rx).await;
    });

    guard.requester = Some(requester);
    guard.node_task = Some(node_task);
    guard.event_task = Some(event_task);
    Ok(Json(serde_json::json!({"ok": true})))
}

async fn stop(State(state): State<Shared>) -> Json<serde_json::Value> {
    let mut guard = state.lock().await;
    stop_locked(&mut guard);
    Json(serde_json::json!({"ok": true}))
}

async fn watch_script(
    State(state): State<Shared>,
    Json(req): Json<WatchScriptRequest>,
) -> Result<Json<serde_json::Value>, (StatusCode, String)> {
    let script = parse_script(&req.script_pubkey_hex)?;
    let mut guard = state.lock().await;
    guard
        .outpoints
        .entry(req.script_pubkey_hex.clone())
        .or_default();
    guard
        .watches
        .insert(req.script_pubkey_hex.clone(), req.clone());
    let requester = guard.requester.clone();
    drop(script);
    drop(guard);

    if let Some(requester) = requester {
        requester
            .rescan_from(req.start_height)
            .map_err(|err| (StatusCode::SERVICE_UNAVAILABLE, err.to_string()))?;
    }
    Ok(Json(serde_json::json!({"ok": true})))
}

async fn best_block(State(state): State<Shared>) -> Result<Json<BlockRef>, (StatusCode, String)> {
    let requester = requester(&state).await?;
    let tip = requester
        .chain_tip()
        .await
        .map_err(|err| (StatusCode::SERVICE_UNAVAILABLE, err.to_string()))?;
    Ok(Json(BlockRef {
        hash_hex: tip.hash.to_string(),
        height: tip.height,
    }))
}

async fn block_hash(
    State(state): State<Shared>,
    Json(req): Json<BlockRef>,
) -> Result<Json<BlockRef>, (StatusCode, String)> {
    if req.height == 0 {
        return Ok(Json(BlockRef {
            hash_hex: REGTEST_GENESIS_HASH.into(),
            height: 0,
        }));
    }

    let requester = requester(&state).await?;
    let header = requester
        .get_header(req.height)
        .await
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
    let guard = state.lock().await;
    let matches = guard
        .matches
        .iter()
        .filter(|m| m.height >= req.start_height && m.height <= req.stop_height)
        .cloned()
        .collect();
    Json(GetMatchesResponse { matches })
}

async fn list_peers(
    State(state): State<Shared>,
) -> Result<Json<ListPeersResponse>, (StatusCode, String)> {
    let requester = requester(&state).await?;
    let peer_info = requester
        .peer_info()
        .await
        .map_err(|err| (StatusCode::SERVICE_UNAVAILABLE, err.to_string()))?;
    let guard = state.lock().await;
    let any_connected = !peer_info.is_empty();
    let peers = guard
        .config
        .peers
        .iter()
        .map(|peer| PeerState {
            id: peer.id.clone(),
            address: peer.address.clone(),
            connected: any_connected,
            banned: false,
            last_error: if any_connected {
                String::new()
            } else {
                "not connected".into()
            },
            best_height: 0,
            best_hash_hex: String::new(),
        })
        .collect();
    Ok(Json(ListPeersResponse { peers }))
}

async fn event_loop(
    state: Shared,
    requester: Requester,
    mut event_rx: tokio::sync::mpsc::UnboundedReceiver<Event>,
) {
    while let Some(event) = event_rx.recv().await {
        if let Event::IndexedFilter(filter) = event {
            let scripts = watched_scripts(&state).await;
            if scripts.is_empty() || !filter.contains_any(scripts.iter()) {
                continue;
            }
            if let Ok(indexed) = requester.get_block(filter.block_hash()).await {
                record_block(
                    &state,
                    indexed.height,
                    indexed.block.block_hash().to_string(),
                    indexed.block,
                )
                .await;
            }
        }
    }
}

async fn watched_scripts(state: &Shared) -> Vec<ScriptBuf> {
    let guard = state.lock().await;
    guard
        .watches
        .keys()
        .filter_map(|hex| parse_script(hex).ok())
        .collect()
}

async fn record_block(state: &Shared, height: u32, block_hash: String, block: bip157::Block) {
    let mut guard = state.lock().await;
    let script_hexes: Vec<String> = guard.watches.keys().cloned().collect();
    for tx in block.txdata {
        let txid = tx.compute_txid().to_string();
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
        "{}:{}:{}:{}:{}:{}",
        tx_match.txid_hex,
        tx_match.block_hash_hex,
        tx_match.height,
        tx_match.kind,
        tx_match.vout,
        tx_match.vin
    );
    if state.seen.insert(key) {
        state.matches.push(tx_match);
    }
}

async fn requester(state: &Shared) -> Result<Requester, (StatusCode, String)> {
    let guard = state.lock().await;
    guard
        .requester
        .clone()
        .ok_or((StatusCode::CONFLICT, "adapter not started".into()))
}

fn stop_locked(state: &mut AdapterState) {
    if let Some(requester) = &state.requester {
        let _ = requester.shutdown();
    }
    if let Some(task) = state.node_task.take() {
        task.abort();
    }
    if let Some(task) = state.event_task.take() {
        task.abort();
    }
    state.requester = None;
    state.node = None;
    state.client = None;
}

fn parse_script(script_hex: &str) -> Result<ScriptBuf, (StatusCode, String)> {
    let bytes = hex::decode(script_hex).map_err(|err| {
        (
            StatusCode::BAD_REQUEST,
            format!("invalid script hex: {err}"),
        )
    })?;
    Ok(ScriptBuf::from_bytes(bytes))
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
            vout: 0,
            vin: 0,
        };
        add_match(&mut state, tx_match.clone());
        add_match(&mut state, tx_match);
        assert_eq!(state.matches.len(), 1);
    }
}
