package harness

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bip157-bip158-test/suite/addresslab"
	"github.com/bip157-bip158-test/suite/api"
	"github.com/bip157-bip158-test/suite/chainlab"
	"github.com/bip157-bip158-test/suite/environment"
	"github.com/bip157-bip158-test/suite/peerlab"
	"github.com/bip157-bip158-test/suite/score"
)

func TestRunWithoutAdapterReportsSkippedAdapter(t *testing.T) {
	summary, err := Run(context.Background(), Options{})
	if err != nil {
		t.Fatalf("run harness: %v", err)
	}
	if summary.Color != score.Green {
		t.Fatalf("internal BIP158 checks should be green, got %s", summary.Color)
	}
	found := false
	for _, result := range summary.Results {
		if result.ID == "adapter.honest_wallet_receive_spend" {
			found = true
			if result.Status != score.Skipped {
				t.Fatalf("adapter scenario should be skipped without adapter, got %s", result.Status)
			}
		}
	}
	if !found {
		t.Fatalf("adapter honest scenario missing from report")
	}
}

func TestRunRejectsUnknownEnvironment(t *testing.T) {
	_, err := Run(context.Background(), Options{Environment: "bogus"})
	if err == nil {
		t.Fatalf("unknown environment unexpectedly succeeded")
	}
}

func TestRunRejectsUnknownAddressLab(t *testing.T) {
	_, err := Run(context.Background(), Options{AddressLab: "bogus"})
	if err == nil {
		t.Fatalf("unknown address lab unexpectedly succeeded")
	}
}

func TestRunRejectsUnknownTorLab(t *testing.T) {
	_, err := Run(context.Background(), Options{TorLab: "bogus"})
	if err == nil {
		t.Fatalf("unknown tor lab unexpectedly succeeded")
	}
}

func TestRunRequiresDistinctIdentities(t *testing.T) {
	_, err := Run(context.Background(), Options{
		Environment:               "ipv6",
		RequireDistinctIdentities: true,
	})
	if err == nil {
		t.Fatalf("shared ipv6 loopback unexpectedly satisfied strict identity mode")
	}
}

func TestDistinctIdentityCapabilityFollowsAllocator(t *testing.T) {
	ipv6, err := environment.Lookup("ipv6")
	if err != nil {
		t.Fatalf("lookup ipv6: %v", err)
	}
	if hasDistinctPeerIdentities(ipv6, addresslab.NewLoopback()) {
		t.Fatalf("loopback ipv6 should not claim distinct identities")
	}
	linux := addresslab.NewLinuxIPRoute(addresslab.LinuxIPRouteOptions{
		Command: &recordingAddressRunner{},
	})
	if !hasDistinctPeerIdentities(ipv6, linux) {
		t.Fatalf("linux-iproute ipv6 should claim distinct identities")
	}
}

func TestWaitForAdapterTipTimesOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := waitForAdapterTip(ctx, nilClient{}, "", 0)
	if err == nil {
		t.Fatalf("expected timeout")
	}
}

func TestWaitForMatchesTimesOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := waitForMatches(ctx, nilClient{}, api.GetMatchesRequest{}, 1)
	if err == nil {
		t.Fatalf("expected timeout")
	}
}

type nilClient struct{}

func (nilClient) PostJSON(context.Context, string, any, any) error { return context.DeadlineExceeded }

type recordingAddressRunner struct{}

func (*recordingAddressRunner) Run(string, ...string) error { return nil }

func TestTranscriptSummaryHandlesEmptyTranscript(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	server := peerlab.NewServer(fixture)
	summary := transcriptSummary("peer-a", server)
	if !strings.Contains(summary, "peer-a transcript: empty") {
		t.Fatalf("unexpected summary: %s", summary)
	}
}

func TestPeerPunishedAfterRequiresObservedBadResponse(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	server := peerlab.NewServer(fixture)
	peers := api.ListPeersResponse{Peers: []api.PeerState{{
		ID:        "bad-peer",
		Connected: false,
		Banned:    false,
		LastError: "not connected",
	}}}

	if ok, evidence := peerPunishedAfter(peers, "bad-peer", server, "cfilter"); ok {
		t.Fatalf("generic disconnect without bad response should not pass: %s", evidence)
	}

	peers.Peers[0].Banned = true
	if ok, evidence := peerPunishedAfter(peers, "bad-peer", server, "cfilter"); !ok {
		t.Fatalf("explicit ban should pass without transcript: %s", evidence)
	}
}
