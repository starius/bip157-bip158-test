using NBitcoin;
using WasabiAdapter;
using Xunit;

namespace WasabiAdapter.Tests;

/// <summary>
/// Unit tests for the harness-facing state kept outside Wasabi's P2P client.
/// </summary>
public sealed class AdapterStateTests
{
    [Fact]
    public void RecordBlockFindsOutputAndSpendMatchesOnce()
    {
        var state = new AdapterState();
        var script = new Script(Convert.FromHexString("00141111111111111111111111111111111111111111"));
        var watch = new WatchScriptRequest(script.ToHex(), 0);
        state.AddWatch(watch, script);

        var receive = Transaction.Create(Network.RegTest);
        receive.Outputs.Add(Money.Satoshis(1000), script);
        var spend = Transaction.Create(Network.RegTest);
        spend.Inputs.Add(new TxIn(new OutPoint(receive.GetHash(), 0)));
        var block = Network.RegTest.Consensus.ConsensusFactory.CreateBlock();
        block.Transactions.Add(receive);
        block.Transactions.Add(spend);
        var blockHash = uint256.One;

        state.RecordBlock(watch.ScriptPubKeyHex, script, 1, blockHash, block);
        state.RecordBlock(watch.ScriptPubKeyHex, script, 1, blockHash, block);

        var matches = state.Matches(new GetMatchesRequest(watch.ScriptPubKeyHex, 0, 2));
        Assert.Equal(2, matches.Length);
        Assert.Contains(matches, x => x.Kind == "output" && x.Vout == 0);
        Assert.Contains(matches, x => x.Kind == "spend" && x.Vin == 0);
    }

    [Fact]
    public void WatchSnapshotDoesNotExposeMutableCursor()
    {
        var state = new AdapterState();
        var script = new Script(Convert.FromHexString("00142222222222222222222222222222222222222222"));
        state.AddWatch(new WatchScriptRequest(script.ToHex(), 7), script);

        var snapshot = state.SnapshotWatches();
        snapshot[0].NextScanHeight = 99;

        Assert.Equal(7u, state.SnapshotWatches()[0].NextScanHeight);
    }

    [Fact]
    public void CapabilitiesAreExplicitIpv4Only()
    {
        var caps = AdapterCapabilities.ClearIPv4Only();

        Assert.Equal(5, caps.Environments.Length);
        Assert.Contains(caps.Environments, x => x.Id == "ipv4" && x.Supported);
        Assert.All(caps.Environments.Where(x => x.Id != "ipv4"), x => Assert.False(x.Supported));
    }
}
