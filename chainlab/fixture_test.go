package chainlab

import "testing"

func TestBuildWalletFixtureHasReceiveAndSpend(t *testing.T) {
	fixture, err := BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	if len(fixture.Blocks) != 3 {
		t.Fatalf("expected genesis plus two mined blocks, got %d", len(fixture.Blocks))
	}
	if len(fixture.Matches) != 2 {
		t.Fatalf("expected receive and spend matches, got %d", len(fixture.Matches))
	}
	if fixture.Matches[0].Kind != "output" {
		t.Fatalf("first match should be output, got %s", fixture.Matches[0].Kind)
	}
	if fixture.Matches[1].Kind != "spend" {
		t.Fatalf("second match should be spend, got %s", fixture.Matches[1].Kind)
	}
}

func TestWalletFixtureFiltersMatchWatchedScript(t *testing.T) {
	fixture, err := BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}

	receive := fixture.Blocks[1]
	matches, err := Contains(receive.Filter.FilterBytes, receive.Block.BlockHash(), fixture.WatchedScript)
	if err != nil {
		t.Fatalf("match receive filter: %v", err)
	}
	if !matches {
		t.Fatalf("receive block filter does not match watched script")
	}

	spend := fixture.Blocks[2]
	matches, err = Contains(spend.Filter.FilterBytes, spend.Block.BlockHash(), fixture.WatchedScript)
	if err != nil {
		t.Fatalf("match spend filter: %v", err)
	}
	if !matches {
		t.Fatalf("spend block filter does not match watched prevout script")
	}
}

func TestBuildLongWalletFixtureCrossesCheckpoints(t *testing.T) {
	fixture, err := BuildLongWalletFixture(DefaultLongChainHeight)
	if err != nil {
		t.Fatalf("build long fixture: %v", err)
	}
	if got := fixture.Blocks[len(fixture.Blocks)-1].Height; got != DefaultLongChainHeight {
		t.Fatalf("tip height = %d, want %d", got, DefaultLongChainHeight)
	}
	if len(fixture.Matches) != 2 {
		t.Fatalf("long fixture should keep wallet matches stable, got %d", len(fixture.Matches))
	}
	for _, height := range []uint32{1000, 2000} {
		block := fixture.Blocks[height]
		if block.Height != height {
			t.Fatalf("block index %d has height %d", height, block.Height)
		}
		if block.Filter.FilterHeader.IsEqual(&fixture.Blocks[height-1].Filter.FilterHeader) {
			t.Fatalf("checkpoint height %d unexpectedly reused previous filter header", height)
		}
	}
}
