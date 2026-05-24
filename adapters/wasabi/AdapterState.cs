using System.Collections.Concurrent;
using NBitcoin;

namespace WasabiAdapter;

/// <summary>
/// Stores harness-facing state that Wasabi's P2P client does not expose
/// directly, such as watched scripts and normalized transaction matches.
/// </summary>
internal sealed class AdapterState
{
    private readonly object gate = new();
    private readonly Dictionary<string, WatchState> watches = new();
    private readonly Dictionary<string, HashSet<OutPoint>> outpoints = new();
    private readonly HashSet<string> seenMatches = new();
    private readonly List<TxMatch> matches = new();

    /// <summary>Last `/configure` request accepted by the adapter.</summary>
    public ConfigureRequest? Config { get; private set; }

    /// <summary>The currently running Wasabi client wrapper.</summary>
    public WasabiP2pClient? Client { get; private set; }

    /// <summary>Records the last known connection error for each peer address.</summary>
    public ConcurrentDictionary<string, string> PeerErrors { get; } = new();

    /// <summary>Replaces the active scenario and clears all wallet state.</summary>
    public async Task ConfigureAsync(ConfigureRequest request)
    {
        WasabiP2pClient? oldClient;
        lock (gate)
        {
            oldClient = Client;
            Client = null;
            Config = request;
            watches.Clear();
            outpoints.Clear();
            seenMatches.Clear();
            matches.Clear();
            PeerErrors.Clear();
        }
        if (oldClient is not null)
        {
            await oldClient.DisposeAsync().ConfigureAwait(false);
        }
    }

    /// <summary>Installs the Wasabi client created for the current scenario.</summary>
    public void SetClient(WasabiP2pClient client)
    {
        lock (gate)
        {
            Client = client;
        }
    }

    /// <summary>Stops the current Wasabi client, if one is running.</summary>
    public async Task StopAsync()
    {
        WasabiP2pClient? oldClient;
        lock (gate)
        {
            oldClient = Client;
            Client = null;
        }
        if (oldClient is not null)
        {
            await oldClient.DisposeAsync().ConfigureAwait(false);
        }
    }

    /// <summary>Adds or replaces one watched script and resets its scan cursor.</summary>
    public void AddWatch(WatchScriptRequest request, Script script)
    {
        lock (gate)
        {
            watches[request.ScriptPubKeyHex] = new WatchState(request, script);
            outpoints.TryAdd(request.ScriptPubKeyHex, []);
        }
    }

    /// <summary>Returns a stable copy of all watches for background scanning.</summary>
    public WatchState[] SnapshotWatches()
    {
        lock (gate)
        {
            return watches.Values.Select(x => x.Clone()).ToArray();
        }
    }

    /// <summary>Advances one watch cursor after filters have been scanned.</summary>
    public void MarkScanned(string scriptHex, uint nextHeight)
    {
        lock (gate)
        {
            if (watches.TryGetValue(scriptHex, out var watch))
            {
                watch.NextScanHeight = Math.Max(watch.NextScanHeight, nextHeight);
            }
        }
    }

    /// <summary>Records output and spend matches from a downloaded block.</summary>
    public void RecordBlock(string scriptHex, Script script, uint height, uint256 blockHash, Block block)
    {
        lock (gate)
        {
            if (!outpoints.TryGetValue(scriptHex, out var watchedOutpoints))
            {
                watchedOutpoints = [];
                outpoints[scriptHex] = watchedOutpoints;
            }

            foreach (var tx in block.Transactions)
            {
                var txid = tx.GetHash();
                for (var vout = 0; vout < tx.Outputs.Count; vout++)
                {
                    if (tx.Outputs[vout].ScriptPubKey != script)
                    {
                        continue;
                    }

                    watchedOutpoints.Add(new OutPoint(txid, vout));
                    AddMatchNoLock(new TxMatch(
                        txid.ToString(),
                        blockHash.ToString(),
                        height,
                        "output",
                        (uint)vout));
                }

                for (var vin = 0; vin < tx.Inputs.Count; vin++)
                {
                    if (!watchedOutpoints.Contains(tx.Inputs[vin].PrevOut))
                    {
                        continue;
                    }

                    AddMatchNoLock(new TxMatch(
                        txid.ToString(),
                        blockHash.ToString(),
                        height,
                        "spend",
                        Vin: (uint)vin));
                }
            }
        }
    }

    /// <summary>Returns matches for a script and inclusive height range.</summary>
    public TxMatch[] Matches(GetMatchesRequest request)
    {
        lock (gate)
        {
            return matches
                .Where(x => x.Height >= request.StartHeight && x.Height <= request.StopHeight)
                .ToArray();
        }
    }

    private void AddMatchNoLock(TxMatch match)
    {
        var key = $"{match.TxIdHex}:{match.BlockHashHex}:{match.Height}:{match.Kind}:{match.Vout}:{match.Vin}";
        if (seenMatches.Add(key))
        {
            matches.Add(match);
        }
    }
}

/// <summary>
/// Mutable scan cursor for one watched script.
/// </summary>
internal sealed class WatchState
{
    public WatchState(WatchScriptRequest request, Script script)
    {
        Request = request;
        Script = script;
        NextScanHeight = request.StartHeight;
    }

    public WatchScriptRequest Request { get; }
    public Script Script { get; }
    public uint NextScanHeight { get; set; }

    public WatchState Clone() => new(Request, Script) { NextScanHeight = NextScanHeight };
}
