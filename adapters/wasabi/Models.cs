using System.Text.Json.Serialization;

namespace WasabiAdapter;

/// <summary>
/// Identifies one Bitcoin P2P peer controlled by the conformance harness.
/// </summary>
internal sealed record PeerConfig(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("address")] string Address,
    [property: JsonPropertyName("trusted")] bool Trusted);

/// <summary>
/// Resets the adapter for one isolated regtest scenario.
/// </summary>
internal sealed record ConfigureRequest(
    [property: JsonPropertyName("network")] string Network,
    [property: JsonPropertyName("data_dir")] string DataDir,
    [property: JsonPropertyName("peers")] PeerConfig[] Peers,
    [property: JsonPropertyName("required_peers")] uint RequiredPeers,
    [property: JsonPropertyName("allow_discovery")] bool AllowDiscovery);

/// <summary>
/// Reports a block hash and height on the adapter's best known chain.
/// </summary>
internal sealed record BlockRef(
    [property: JsonPropertyName("hash_hex")] string HashHex,
    [property: JsonPropertyName("height")] uint Height);

/// <summary>
/// Registers one scriptPubKey to scan for from a given block height.
/// </summary>
internal sealed record WatchScriptRequest(
    [property: JsonPropertyName("script_pubkey_hex")] string ScriptPubKeyHex,
    [property: JsonPropertyName("start_height")] uint StartHeight);

/// <summary>
/// Normalized wallet-relevance result returned to the harness.
/// </summary>
internal sealed record TxMatch(
    [property: JsonPropertyName("txid_hex")] string TxIdHex,
    [property: JsonPropertyName("block_hash_hex")] string BlockHashHex,
    [property: JsonPropertyName("height")] uint Height,
    [property: JsonPropertyName("kind")] string Kind,
    [property: JsonPropertyName("vout")] uint Vout = 0,
    [property: JsonPropertyName("vin")] uint Vin = 0);

/// <summary>
/// Queries matches for a watched script over an inclusive height range.
/// </summary>
internal sealed record GetMatchesRequest(
    [property: JsonPropertyName("script_pubkey_hex")] string ScriptPubKeyHex,
    [property: JsonPropertyName("start_height")] uint StartHeight,
    [property: JsonPropertyName("stop_height")] uint StopHeight);

/// <summary>
/// Contains all matches known to the adapter for a request.
/// </summary>
internal sealed record GetMatchesResponse(
    [property: JsonPropertyName("matches")] TxMatch[] Matches);

/// <summary>
/// Exposes the adapter's current view of one harness peer.
/// </summary>
internal sealed record PeerState(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("address")] string Address,
    [property: JsonPropertyName("connected")] bool Connected,
    [property: JsonPropertyName("banned")] bool Banned,
    [property: JsonPropertyName("last_error")] string LastError = "",
    [property: JsonPropertyName("best_height")] uint BestHeight = 0,
    [property: JsonPropertyName("best_hash_hex")] string BestHashHex = "");

/// <summary>
/// Lists every harness peer the adapter can report.
/// </summary>
internal sealed record ListPeersResponse(
    [property: JsonPropertyName("peers")] PeerState[] Peers);

/// <summary>
/// Describes support for one address environment.
/// </summary>
internal sealed record EnvironmentCapability(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("supported")] bool Supported,
    [property: JsonPropertyName("reason")] string Reason = "");

/// <summary>
/// Reports the address environments this adapter is willing to run.
/// </summary>
internal sealed record CapabilitiesResponse(
    [property: JsonPropertyName("environments")] EnvironmentCapability[] Environments);

/// <summary>
/// Minimal liveness and state response.
/// </summary>
internal sealed record HealthResponse(
    [property: JsonPropertyName("alive")] bool Alive,
    [property: JsonPropertyName("status")] string Status);
