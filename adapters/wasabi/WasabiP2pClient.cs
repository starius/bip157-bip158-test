using System.Net;
using NBitcoin;
using NBitcoin.Protocol;
using WalletWasabi.Backend.Models;
using WalletWasabi.BitcoinP2p;
using WalletWasabi.Blockchain.Blocks;
using WalletWasabi.Helpers;
using WalletWasabi.Services;
using WalletWasabi.Stores;
using WalletWasabi.Wallets;
using ChainHeight = WalletWasabi.Models.Height.ChainHeight;

namespace WasabiAdapter;

/// <summary>
/// Owns a minimal Wasabi P2P compact-filter client for one harness scenario.
/// </summary>
internal sealed class WasabiP2pClient : IAsyncDisposable
{
    private const int FilterBatchSize = 100;
    private readonly AdapterState adapterState;
    private readonly CancellationTokenSource stop = new();
    private readonly object peersGate = new();
    private readonly List<Node> nodes = new();
    private readonly Dictionary<string, Node> peerNodes = new(StringComparer.Ordinal);
    private readonly Task networkTask;
    private readonly Task filterTask;
    private readonly Task scanTask;
    private readonly Timer tickTimer;
    private readonly BlockProvider blockProvider;

    /// <summary>Bitcoin block headers tracked by NBitcoin's chain behavior.</summary>
    public ConcurrentChain BlockHeaders { get; }

    /// <summary>Compact-filter headers tracked by Wasabi's P2P behavior.</summary>
    public FilterHeaderChain FilterHeaders { get; }

    /// <summary>Harness configuration used for this client instance.</summary>
    public ConfigureRequest Config { get; }

    private WasabiP2pClient(
        AdapterState adapterState,
        ConfigureRequest config,
        EventBus eventBus,
        ConcurrentChain blockHeaders,
        FilterHeaderChain filterHeaders,
        FilterStore filterStore,
        NodesGroup nodesGroup,
        CompactFilterBehavior.FilterSynchronizationState filterSyncState)
    {
        this.adapterState = adapterState;
        Config = config;
        EventBus = eventBus;
        BlockHeaders = blockHeaders;
        FilterHeaders = filterHeaders;
        FilterStore = filterStore;
        NodesGroup = nodesGroup;
        FilterSyncState = filterSyncState;
        blockProvider = BlockProviders.P2pBlockProvider(new P2PNodesManager(Network.RegTest, nodesGroup));
        tickTimer = new Timer(
            _ => EventBus.Publish(new Tick(DateTime.UtcNow)),
            null,
            TimeSpan.FromMilliseconds(250),
            TimeSpan.FromMilliseconds(500));
        networkTask = Task.Run(RunNetworkAsync);
        filterTask = Task.Run(RunFilterSyncAsync);
        scanTask = Task.Run(RunWatchScannerAsync);
    }

    private EventBus EventBus { get; }
    private FilterStore FilterStore { get; }
    private NodesGroup NodesGroup { get; }
    private CompactFilterBehavior.FilterSynchronizationState FilterSyncState { get; }

    /// <summary>Creates and initializes a Wasabi P2P client for regtest.</summary>
    public static async Task<WasabiP2pClient> CreateAsync(
        AdapterState adapterState,
        ConfigureRequest config,
        CancellationToken cancellationToken)
    {
        Directory.CreateDirectory(config.DataDir);

        var eventBus = new EventBus();
        var blockHeaders = new ConcurrentChain(Network.RegTest);
        var filterHeaders = new FilterHeaderChain();
        var filterStore = new FilterStore(
            Path.Combine(config.DataDir, "IndexStore"),
            Network.RegTest,
            filterHeaders,
            eventBus);

        await filterStore.InitializeAsync(ChainHeight.Genesis, cancellationToken).ConfigureAwait(false);
        var filterTip = filterStore.GetTip()?.Header.Height ?? 0;
        var filterSyncState = new CompactFilterBehavior.FilterSynchronizationState(
            blockHeaders,
            filterHeaders,
            filterTip);

        var nodesGroup = new NodesGroup(Network.RegTest);
        return new WasabiP2pClient(
            adapterState,
            config,
            eventBus,
            blockHeaders,
            filterHeaders,
            filterStore,
            nodesGroup,
            filterSyncState);
    }

    /// <summary>Returns the best known block hash and height.</summary>
    public BlockRef BestBlock()
    {
        var tip = BlockHeaders.Tip;
        if (tip is null)
        {
            return new BlockRef(Network.RegTest.GenesisHash.ToString(), 0);
        }
        return new BlockRef(tip.HashBlock.ToString(), (uint)tip.Height);
    }

    /// <summary>Returns the best-chain block hash at a height, if known.</summary>
    public BlockRef? BlockHash(uint height)
    {
        if (height == 0)
        {
            return new BlockRef(Network.RegTest.GenesisHash.ToString(), 0);
        }

        var block = BlockHeaders.GetBlock((int)height);
        return block is null ? null : new BlockRef(block.HashBlock.ToString(), height);
    }

    /// <summary>Returns peer state for every harness peer.</summary>
    public PeerState[] ListPeers()
    {
        var best = BestBlock();
        Dictionary<string, Node> currentPeers;
        lock (peersGate)
        {
            currentPeers = new Dictionary<string, Node>(peerNodes, StringComparer.Ordinal);
        }

        return Config.Peers.Select(peer =>
        {
            var isConnected = currentPeers.TryGetValue(peer.Address, out var node) && node.IsConnected;
            adapterState.PeerErrors.TryGetValue(peer.Address, out var lastError);
            return new PeerState(
                peer.Id,
                peer.Address,
                isConnected,
                Banned: false,
                LastError: isConnected ? "" : lastError ?? "not connected",
                BestHeight: best.Height,
                BestHashHex: best.HashHex);
        }).ToArray();
    }

    /// <summary>Stops network activity and releases Wasabi storage handles.</summary>
    public async ValueTask DisposeAsync()
    {
        stop.Cancel();
        tickTimer.Dispose();
        Node[] currentNodes;
        lock (peersGate)
        {
            currentNodes = nodes.ToArray();
        }
        foreach (var node in currentNodes)
        {
            node.DisconnectAsync("adapter stopped");
        }
        NodesGroup.Disconnect();
        NodesGroup.Dispose();
        await FilterStore.DisposeAsync().ConfigureAwait(false);
        try
        {
            await Task.WhenAll(networkTask, filterTask, scanTask).WaitAsync(TimeSpan.FromSeconds(5))
                .ConfigureAwait(false);
        }
        catch (Exception)
        {
            // Shutdown is best-effort. Individual loops observe the cancellation
            // token and may already be waiting inside NBitcoin internals.
        }
        stop.Dispose();
    }

    private async Task RunNetworkAsync()
    {
        foreach (var peer in Config.Peers)
        {
            if (stop.IsCancellationRequested)
            {
                return;
            }

            try
            {
                var endpoint = ParseEndpoint(peer.Address);
                var node = Node.Connect(Network.RegTest, endpoint);
                node.Behaviors.Add(new BlockHeadersChainBehavior(BlockHeaders, FilterHeaders, EventBus));
                node.Behaviors.Add(new CompactFilterBehavior(FilterSyncState, BlockHeaders, EventBus));
                node.StateChanged += (_, _) =>
                {
                    if (!node.IsConnected)
                    {
                        adapterState.PeerErrors[peer.Address] = "disconnected";
                    }
                };
                using var handshakeTimeout = CancellationTokenSource.CreateLinkedTokenSource(stop.Token);
                handshakeTimeout.CancelAfter(TimeSpan.FromSeconds(10));
                node.VersionHandshake(handshakeTimeout.Token);
                adapterState.PeerErrors.TryRemove(peer.Address, out _);
                lock (peersGate)
                {
                    nodes.Add(node);
                    peerNodes[peer.Address] = node;
                }
                NodesGroup.ConnectedNodes.Add(node);
            }
            catch (Exception ex)
            {
                adapterState.PeerErrors[peer.Address] = ex.Message;
            }
        }

        while (!stop.Token.IsCancellationRequested)
        {
            await Task.Delay(TimeSpan.FromSeconds(1), stop.Token).ConfigureAwait(false);
        }
    }

    private async Task RunFilterSyncAsync()
    {
        var provider = FilterProviders.CreateBitcoinP2pFilterProvider(
            FilterHeaders,
            BlockHeaders,
            FilterSyncState);
        var handler = Synchronizer.CreateFilterGenerator(provider, FilterStore, FilterHeaders, EventBus);

        while (!stop.Token.IsCancellationRequested)
        {
            try
            {
                await handler(Unit.Instance, stop.Token).ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (stop.IsCancellationRequested)
            {
                return;
            }
            catch
            {
                await Task.Delay(TimeSpan.FromSeconds(1), stop.Token).ConfigureAwait(false);
            }
        }
    }

    private async Task RunWatchScannerAsync()
    {
        while (!stop.Token.IsCancellationRequested)
        {
            foreach (var watch in adapterState.SnapshotWatches())
            {
                await ScanWatchAsync(watch, stop.Token).ConfigureAwait(false);
            }
            await Task.Delay(TimeSpan.FromMilliseconds(500), stop.Token).ConfigureAwait(false);
        }
    }

    private async Task ScanWatchAsync(WatchState watch, CancellationToken cancellationToken)
    {
        var filters = await FilterStore.FetchBatchAsync(
            watch.NextScanHeight,
            FilterBatchSize,
            cancellationToken).ConfigureAwait(false);

        if (filters.Length == 0)
        {
            return;
        }

        uint nextHeight = watch.NextScanHeight;
        foreach (FilterModel filter in filters)
        {
            nextHeight = Math.Max(nextHeight, filter.Header.Height + 1);
            if (filter.Header.Height < watch.Request.StartHeight)
            {
                continue;
            }

            var elements = new[] { watch.Script.ToBytes() };
            if (!filter.Filter.MatchAny(elements, filter.FilterKey))
            {
                continue;
            }

            var block = await blockProvider(filter.Header.BlockHash, cancellationToken)
                .ConfigureAwait(false);
            if (block is not null)
            {
                adapterState.RecordBlock(
                    watch.Request.ScriptPubKeyHex,
                    watch.Script,
                    filter.Header.Height,
                    filter.Header.BlockHash,
                    block);
            }
        }
        adapterState.MarkScanned(watch.Request.ScriptPubKeyHex, nextHeight);
    }

    /// <summary>Parses the host:port endpoint form used by the harness.</summary>
    internal static EndPoint ParseEndpoint(string address)
    {
        if (IPEndPoint.TryParse(address, out var ipEndpoint))
        {
            return ipEndpoint;
        }

        var parts = address.Split(':', 2);
        if (parts.Length == 2 && int.TryParse(parts[1], out var port))
        {
            return new DnsEndPoint(parts[0], port);
        }

        throw new FormatException($"invalid peer endpoint: {address}");
    }
}
