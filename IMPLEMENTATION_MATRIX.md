# BIP157/BIP158 Implementation Matrix

| Implementation | Overall |
| --- | --- |
| `no-adapter` | `green` |
| `fake` | `green` |
| `kyoto` | `red` |
| `wasabi` | `red` |
| `neutrino` | `red` |
| `nakamoto` | `red` |

| Test | BIP Status | no-adapter | fake | kyoto | wasabi | neutrino | nakamoto |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `adapter.honest_wallet_receive_spend` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `fail` | `fail` |
| `bip157.bad_cfcheckpt_response` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `bip157.bad_cfheaders_prev_header` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `bip157.cfheaders_order_and_checkpoint_boundaries` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `fail` | `fail` |
| `bip157.cfilter_block_hash_sequence_mismatch` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `bip157.conflict_one_honest_one_liar` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `bip157.direct_bad_cfilter_ban` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `bip157.empty_cfheaders_response` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `bip157.large_batch_progress_timeout` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `fail` | `fail` |
| `bip157.malformed_gcs_filter_payload` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `bip157.self_consistent_eclipse` | `SHOULD` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `bip157.wrong_filter_type_response` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `bip158.coinbase_input_excluded` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
| `bip158.coinbase_output_included` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
| `bip158.empty_filter_wire_forms` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
| `bip158.full_script_not_pushdata` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
| `bip158.op_return_excluded` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
| `bip158.prevout_legacy_included` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
| `bip158.prevout_p2sh_included` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
| `bip158.prevout_p2wsh_included` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
| `bip158.prevout_taproot_included` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
| `bip158.zero_element_serialization` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
| `blocks.invalid_downloaded_block_rejected` | `MUST` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `chain.long_checkpointed_header_sync` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `fail` | `fail` |
| `kyoto.live_reorg` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `kyoto.live_reorg_additional_sync` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `kyoto.stop_reorg_resync` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `kyoto.stop_reorg_start_on_orphan` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `kyoto.stop_reorg_two_resync` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `kyoto.tx_can_broadcast` | `SHOULD` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `kyoto.various_client_methods` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `fail` | `fail` |
| `kyoto.whitelist_only_sync` | `SHOULD` | `skipped` | `pass` | `pass` | `pass` | `skipped` | `skipped` |
| `network.outage_block_download` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `fail` | `fail` |
| `network.outage_filter_headers` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `fail` | `fail` |
| `network.restricted_connect_no_discovery` | `OTHER` | `skipped` | `pass` | `pass` | `pass` | `skipped` | `skipped` |
| `neutrino.blockmanager_initial_interval.permute_false_partial_false_repeat_false` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.blockmanager_initial_interval.permute_false_partial_false_repeat_true` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.blockmanager_initial_interval.permute_false_partial_true_repeat_false` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.blockmanager_initial_interval.permute_false_partial_true_repeat_true` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.blockmanager_initial_interval.permute_true_partial_false_repeat_false` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.blockmanager_initial_interval.permute_true_partial_false_repeat_true` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.blockmanager_initial_interval.permute_true_partial_true_repeat_false` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.blockmanager_initial_interval.permute_true_partial_true_repeat_true` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.blockmanager_invalid_interval.interval_misaligned` | `MUST` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.blockmanager_invalid_interval.interval_misaligned_partial` | `MUST` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.blockmanager_invalid_interval.invalid_prev_header` | `MUST` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.blockmanager_invalid_interval.wrong_genesis` | `MUST` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.blockmanager_invalid_interval.wrong_genesis_partial` | `MUST` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfcheckpt_sanity.case_1` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfcheckpt_sanity.case_2` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfcheckpt_sanity.case_3` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfcheckpt_sanity.case_4` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfcheckpt_sanity.case_5` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfcheckpt_sanity.case_6` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfcheckpt_sanity.case_7` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfcheckpt_sanity.case_8` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfcheckpt_sanity.case_9` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfheaders_mismatch.case_1` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfheaders_mismatch.case_2` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfheaders_mismatch.case_3` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfheaders_mismatch.case_4` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfheaders_mismatch.case_5` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfheaders_mismatch.case_6` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.cfheaders_mismatch.case_7` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.detect_bad_peers.filter_hash_mismatch` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.detect_bad_peers.unresponsive_peer` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.handle_headers.valid_then_scrambled` | `MUST` | `skipped` | `pass` | `fail` | `fail` | `pass` | `fail` |
| `neutrino.import_then_p2p_sync` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.resolve_filter_mismatch.case_1` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_10` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_11` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_12` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_13` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_2` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_3` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_4` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_5` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_6` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_7` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_8` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.resolve_filter_mismatch.case_9` | `SHOULD` | `skipped` | `pass` | `fail` | `fail` | `fail` | `fail` |
| `neutrino.sync_with_headers_import` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.sync_without_headers_import.initial_sync` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `fail` | `fail` |
| `neutrino.sync_without_headers_import.long_rescan_start` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `skipped` | `skipped` |
| `neutrino.sync_without_headers_import.one_shot_rescan` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `skipped` | `skipped` |
| `neutrino.sync_without_headers_import.random_blocks_filters` | `MUST` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` | `skipped` |
| `neutrino.sync_without_headers_import.rescan_results` | `MUST` | `skipped` | `pass` | `pass` | `pass` | `skipped` | `skipped` |
| `neutrino.verify_basic_filter.standard_scripts` | `MUST` | `pass` | `pass` | `pass` | `pass` | `pass` | `pass` |
