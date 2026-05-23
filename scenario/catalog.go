// Package scenario lists the conformance scenarios the harness knows about.
//
// The catalog deliberately includes scenarios equivalent to the existing Kyoto
// and Neutrino tests. That baseline prevents this suite from becoming weaker
// than the implementation-specific test suites it is meant to replace.
package scenario

import "github.com/bip157-bip158-test/suite/score"

// Source names where a scenario came from. Baseline scenarios map to tests that
// already exist in Kyoto or Neutrino; conformance scenarios are stronger
// black-box checks added by this suite.
const (
	SourceKyotoBaseline    = "kyoto-baseline"
	SourceNeutrinoBaseline = "neutrino-baseline"
	SourceConformance      = "conformance"
)

// Definition describes one scenario without containing its executable logic.
// Keeping the metadata separate lets reports include skipped and not-yet-run
// cases, which is important for an honest conformance score.
type Definition struct {
	ID          string
	Title       string
	Level       score.Level
	Source      string
	Description string
}

// Catalog returns every scenario currently tracked by the suite.
func Catalog() []Definition {
	var defs []Definition
	defs = append(defs, kyotoBaseline()...)
	defs = append(defs, neutrinoBaseline()...)
	defs = append(defs, conformanceScenarios()...)
	return defs
}

func kyotoBaseline() []Definition {
	return []Definition{
		{ID: "kyoto.live_reorg", Title: "live reorg", Level: score.Must, Source: SourceKyotoBaseline},
		{ID: "kyoto.live_reorg_additional_sync", Title: "live reorg plus additional sync", Level: score.Must, Source: SourceKyotoBaseline},
		{ID: "kyoto.various_client_methods", Title: "client best block, header, peer, and unknown hash APIs", Level: score.Must, Source: SourceKyotoBaseline},
		{ID: "kyoto.stop_reorg_resync", Title: "cold restart after one-block reorg", Level: score.Must, Source: SourceKyotoBaseline},
		{ID: "kyoto.stop_reorg_two_resync", Title: "cold restart after two-block reorg", Level: score.Must, Source: SourceKyotoBaseline},
		{ID: "kyoto.stop_reorg_start_on_orphan", Title: "restart from orphaned persisted tip", Level: score.Must, Source: SourceKyotoBaseline},
		{ID: "kyoto.tx_can_broadcast", Title: "transaction broadcast smoke test", Level: score.Should, Source: SourceKyotoBaseline},
		{ID: "kyoto.whitelist_only_sync", Title: "whitelist-only peer selection", Level: score.Should, Source: SourceKyotoBaseline},
	}
}

func neutrinoBaseline() []Definition {
	defs := []Definition{
		{ID: "neutrino.sync_with_headers_import", Title: "headers import then sync", Level: score.Must, Source: SourceNeutrinoBaseline},
		{ID: "neutrino.import_then_p2p_sync", Title: "single session import then P2P sync", Level: score.Must, Source: SourceNeutrinoBaseline},
		{ID: "neutrino.sync_without_headers_import.initial_sync", Title: "multi-peer initial sync", Level: score.Must, Source: SourceNeutrinoBaseline},
		{ID: "neutrino.sync_without_headers_import.one_shot_rescan", Title: "one-shot rescan", Level: score.Must, Source: SourceNeutrinoBaseline},
		{ID: "neutrino.sync_without_headers_import.long_rescan_start", Title: "long-running rescan start", Level: score.Must, Source: SourceNeutrinoBaseline},
		{ID: "neutrino.sync_without_headers_import.random_blocks_filters", Title: "random block and filter fetch order", Level: score.Must, Source: SourceNeutrinoBaseline},
		{ID: "neutrino.sync_without_headers_import.rescan_results", Title: "long-running rescan results", Level: score.Must, Source: SourceNeutrinoBaseline},
		{ID: "neutrino.handle_headers.valid_then_scrambled", Title: "valid headers accepted and scrambled headers disconnected", Level: score.Must, Source: SourceNeutrinoBaseline},
		{ID: "neutrino.verify_basic_filter.standard_scripts", Title: "basic filter verification for standard scripts", Level: score.Must, Source: SourceNeutrinoBaseline},
	}

	for _, suffix := range []string{
		"permute_false_partial_false_repeat_false",
		"permute_false_partial_false_repeat_true",
		"permute_false_partial_true_repeat_false",
		"permute_false_partial_true_repeat_true",
		"permute_true_partial_false_repeat_false",
		"permute_true_partial_false_repeat_true",
		"permute_true_partial_true_repeat_false",
		"permute_true_partial_true_repeat_true",
	} {
		defs = append(defs, Definition{
			ID:     "neutrino.blockmanager_initial_interval." + suffix,
			Title:  "checkpointed filter-header interval " + suffix,
			Level:  score.Must,
			Source: SourceNeutrinoBaseline,
		})
	}

	for _, suffix := range []string{
		"wrong_genesis",
		"wrong_genesis_partial",
		"interval_misaligned",
		"interval_misaligned_partial",
		"invalid_prev_header",
	} {
		defs = append(defs, Definition{
			ID:     "neutrino.blockmanager_invalid_interval." + suffix,
			Title:  "invalid filter-header interval " + suffix,
			Level:  score.Must,
			Source: SourceNeutrinoBaseline,
		})
	}

	for _, suffix := range []string{"unresponsive_peer", "filter_hash_mismatch"} {
		defs = append(defs, Definition{
			ID:     "neutrino.detect_bad_peers." + suffix,
			Title:  "detect bad peers: " + suffix,
			Level:  score.Should,
			Source: SourceNeutrinoBaseline,
		})
	}

	for i := 1; i <= 7; i++ {
		defs = append(defs, Definition{
			ID:     "neutrino.cfheaders_mismatch.case_" + itoa(i),
			Title:  "compact-filter-header mismatch case " + itoa(i),
			Level:  score.Should,
			Source: SourceNeutrinoBaseline,
		})
	}

	for i := 1; i <= 9; i++ {
		defs = append(defs, Definition{
			ID:     "neutrino.cfcheckpt_sanity.case_" + itoa(i),
			Title:  "compact-filter checkpoint sanity case " + itoa(i),
			Level:  score.Should,
			Source: SourceNeutrinoBaseline,
		})
	}

	for i := 1; i <= 13; i++ {
		defs = append(defs, Definition{
			ID:     "neutrino.resolve_filter_mismatch.case_" + itoa(i),
			Title:  "resolve filter mismatch from block case " + itoa(i),
			Level:  score.Should,
			Source: SourceNeutrinoBaseline,
		})
	}

	return defs
}

func conformanceScenarios() []Definition {
	return []Definition{
		{ID: "bip158.coinbase_output_included", Title: "coinbase output is included in basic filter", Level: score.Must, Source: SourceConformance},
		{ID: "bip158.coinbase_input_excluded", Title: "coinbase input script is excluded from basic filter", Level: score.Must, Source: SourceConformance},
		{ID: "bip158.prevout_legacy_included", Title: "legacy input prevout script is included", Level: score.Must, Source: SourceConformance},
		{ID: "bip158.prevout_p2sh_included", Title: "P2SH input prevout script is included", Level: score.Must, Source: SourceConformance},
		{ID: "bip158.prevout_p2wsh_included", Title: "P2WSH input prevout script is included", Level: score.Must, Source: SourceConformance},
		{ID: "bip158.prevout_taproot_included", Title: "taproot input prevout script is included", Level: score.Must, Source: SourceConformance},
		{ID: "bip158.op_return_excluded", Title: "OP_RETURN outputs are excluded", Level: score.Must, Source: SourceConformance},
		{ID: "bip158.zero_element_serialization", Title: "zero-element filter is one zero byte", Level: score.Must, Source: SourceConformance},
		{ID: "bip158.full_script_not_pushdata", Title: "basic filters match full scripts rather than pushed data", Level: score.Must, Source: SourceConformance},
		{ID: "bip158.empty_filter_wire_forms", Title: "empty filters use canonical wire form", Level: score.Must, Source: SourceConformance},
		{ID: "chain.long_checkpointed_header_sync", Title: "long chain crosses compact-filter checkpoints", Level: score.Must, Source: SourceConformance},
		{ID: "bip157.large_batch_progress_timeout", Title: "large compact-filter batches make progress", Level: score.Must, Source: SourceConformance},
		{ID: "bip157.cfheaders_order_and_checkpoint_boundaries", Title: "cfheaders ordering and checkpoint boundaries are handled", Level: score.Must, Source: SourceConformance},
		{ID: "bip157.wrong_filter_type_response", Title: "wrong filter type responses are rejected or punished", Level: score.Should, Source: SourceConformance},
		{ID: "bip157.bad_cfcheckpt_response", Title: "bad compact-filter checkpoint response is rejected or punished", Level: score.Should, Source: SourceConformance},
		{ID: "bip157.bad_cfheaders_prev_header", Title: "bad compact-filter previous header is rejected or punished", Level: score.Should, Source: SourceConformance},
		{ID: "bip157.conflict_one_honest_one_liar", Title: "one honest and one liar filter-header conflict", Level: score.Should, Source: SourceConformance},
		{ID: "bip157.direct_bad_cfilter_ban", Title: "bad direct cfilter response is punished", Level: score.Should, Source: SourceConformance},
		{ID: "bip157.self_consistent_eclipse", Title: "self-consistent malicious filter chain is reported as trust limitation", Level: score.Should, Source: SourceConformance},
		{ID: "network.outage_filter_headers", Title: "temporary outage during filter-header sync recovers", Level: score.Must, Source: SourceConformance},
		{ID: "network.outage_block_download", Title: "temporary outage during block download recovers", Level: score.Must, Source: SourceConformance},
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
