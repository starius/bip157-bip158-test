package harness

import (
	"context"
	"testing"
	"time"

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

func TestWaitForAdapterTipTimesOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := waitForAdapterTip(ctx, nilClient{}, "", 0)
	if err == nil {
		t.Fatalf("expected timeout")
	}
}

type nilClient struct{}

func (nilClient) PostJSON(context.Context, string, any, any) error { return context.DeadlineExceeded }
