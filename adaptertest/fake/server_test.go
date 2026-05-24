package fake

import (
	"context"
	"encoding/hex"
	"net/http/httptest"
	"testing"

	"github.com/bip157-bip158-test/suite/api"
	"github.com/bip157-bip158-test/suite/chainlab"
	"github.com/bip157-bip158-test/suite/harness"
	"github.com/bip157-bip158-test/suite/score"
)

func TestFakeAdapterReportsFixtureMatches(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	server := httptest.NewServer(NewServer(fixture).Handler())
	defer server.Close()

	client := api.NewClient(server.URL)
	if err := client.PostJSON(context.Background(), "/configure", api.ConfigureRequest{
		Network: "regtest",
		Peers: []api.PeerConfig{{
			ID:      "peer-a",
			Address: "127.0.0.1:18444",
		}},
	}, nil); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := client.PostJSON(context.Background(), "/start", map[string]string{}, nil); err != nil {
		t.Fatalf("start: %v", err)
	}

	scriptHex := hex.EncodeToString(fixture.WatchedScript)
	if err := client.PostJSON(context.Background(), "/watch-script", api.WatchScriptRequest{
		ScriptPubKeyHex: scriptHex,
	}, nil); err != nil {
		t.Fatalf("watch script: %v", err)
	}

	var matches api.GetMatchesResponse
	if err := client.PostJSON(context.Background(), "/matches", api.GetMatchesRequest{
		ScriptPubKeyHex: scriptHex,
		StopHeight:      2,
	}, &matches); err != nil {
		t.Fatalf("matches: %v", err)
	}
	if len(matches.Matches) != len(fixture.Matches) {
		t.Fatalf("got %d matches, want %d", len(matches.Matches), len(fixture.Matches))
	}

	var peers api.ListPeersResponse
	if err := client.PostJSON(context.Background(), "/list-peers", map[string]string{}, &peers); err != nil {
		t.Fatalf("list peers: %v", err)
	}
	if len(peers.Peers) != 1 || !peers.Peers[0].Connected {
		t.Fatalf("peer state not connected: %+v", peers.Peers)
	}
}

func TestFakeAdapterSuppressesMatchesForInvalidBlockScenario(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	server := httptest.NewServer(NewServer(fixture).Handler())
	defer server.Close()

	client := api.NewClient(server.URL)
	if err := client.PostJSON(context.Background(), "/configure", api.ConfigureRequest{
		Network: "regtest",
		Peers: []api.PeerConfig{{
			ID:      "bad-block",
			Address: "127.0.0.1:18444",
		}},
	}, nil); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := client.PostJSON(context.Background(), "/start", map[string]string{}, nil); err != nil {
		t.Fatalf("start: %v", err)
	}

	scriptHex := hex.EncodeToString(fixture.WatchedScript)
	if err := client.PostJSON(context.Background(), "/watch-script", api.WatchScriptRequest{
		ScriptPubKeyHex: scriptHex,
	}, nil); err != nil {
		t.Fatalf("watch script: %v", err)
	}

	var matches api.GetMatchesResponse
	if err := client.PostJSON(context.Background(), "/matches", api.GetMatchesRequest{
		ScriptPubKeyHex: scriptHex,
		StopHeight:      2,
	}, &matches); err != nil {
		t.Fatalf("matches: %v", err)
	}
	if len(matches.Matches) != 0 {
		t.Fatalf("invalid block scenario should not report fake matches")
	}
}

func TestHarnessPassesAgainstFakeAdapter(t *testing.T) {
	fixture, err := chainlab.BuildLongWalletFixture(chainlab.DefaultLongChainHeight)
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	server := httptest.NewServer(NewServer(fixture).Handler())
	defer server.Close()

	summary, err := harness.Run(context.Background(), harness.Options{
		AdapterURL: server.URL,
		DataDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatalf("run harness: %v", err)
	}
	if summary.Color != score.Green {
		t.Fatalf("fake adapter should produce a green implemented subset, got %s", summary.Color)
	}
}
