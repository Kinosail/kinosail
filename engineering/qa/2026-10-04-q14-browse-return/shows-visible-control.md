## Q14 Shows fixture uses the visible phone episode control

Run [37246009347](https://github.com/Kinosail/kinosail/actions/runs/37246009347) at ac71f9b27b6d2db6d58228843d88ae82322654dd passed direct Play. Its details-and-episode case reached the real Show details page, then timed out at the episode-preview click. Return assertions were not reached.

The original case uses a 390px viewport. The established stylesheet hides episode-preview at widths up to 700px and retains the visible episode-ledger links. The repaired fixture clicks that actual mobile control. The original failed artifact remains historical evidence of a fixture prerequisite mismatch, not product RED.

This changes one test interaction. Case names, original data, Show query, profile, focus and position capture, Player and Back actions, all return assertions, deadlines, retry-zero policy and gates remain unchanged. No CSS or product change is made. Both original Shows journeys require fresh hosted execution and exact source admission.
