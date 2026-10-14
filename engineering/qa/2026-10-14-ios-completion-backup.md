# iOS completion history backup policy

Normal playback can record a completion before the offline catalog has been saved.
`OfflineCatalogStore.saveCompletions` creates its directory without the backup exclusion applied by `save`.
A deferred or interrupted completion operation can leave history in that directory.
File protection does not replace the backup exclusion. No backup transfer or disclosure has been observed.

Apple recommends applying the exclusion to the directory that contains related files.
See [Optimizing Your App's Data for iCloud Backup](https://developer.apple.com/documentation/foundation/optimizing-your-app-s-data-for-icloud-backup).

## Failure paths and verification plan

This contract is written before the regression and production repair.
Existing browser and native journeys do not inspect this directory's backup metadata.
Retain one isolated native storage contract for that gap.

- A first completion can create an unexcluded directory before a catalog save.
- Empty reads must not create storage or hide the first-write failure.
- Completion data must survive reopening the store with the expected item, next item, and quality.
- Removing a completion must preserve the exclusion, including when an existing directory has lost it.
- Reading existing completion history must repair a missing exclusion, including a dormant journal from an older app version.
- The existing catalog writer is a positive control for the simulator's backup attribute support.
- A missing test selection, skipped test, wrong platform, or duplicate execution must fail the focused receipt gate.
- Test setup must refuse an existing root. It must never remove existing simulator data to manufacture a fresh state.
- Directory validation, creation, or exclusion failure must prevent a new payload write. Diagnostics must omit paths, content, and profile identifiers.

Run the focused contract first in the GitHub-hosted native job's fresh iOS app container.
The preceding native build does not install or launch the app.
Then run the complete iOS and tvOS suites on the same hosted simulators.
The opt-in test flag is restricted to the first focused command.
Require the root to be absent before empty reads or writes; remove only the root this test then owns.
Retain the baseline failure before changing production code.

Retain the exact revision, command, fixture source, xcresult summaries, and archive hashes.
Use the same contract after repair, with one passing test and no skips.
This verifies simulator filesystem metadata and persistence, not physical iCloud backup selection or transport.
Local builds, app launches, and device operations remain held.

## Accepted pre-repair reproduction

Revision `4fb2d4a5bfa4de9edcedd38c7f5034ba19d6d208` runs the focused test once, with no skips.
Run `38522246479`, job `115714452624`, fails only the three backup assertions at lines 27, 44, and 48.
These cover the first completion, a legacy journal read, and completion removal.
The absent-root prerequisites, persisted contents, existing catalog control, blocked-write preservation, and final cleanup succeed.
The retained test-detail tree identifies all three failed assertions; the summary groups their identical messages.
The complete suites still pass: 336 iOS tests with two skips, and 322 tvOS tests with one skip.
Both suites report zero runtime warnings. The iOS opt-in backup test is skipped only in the later unrestricted invocation.

Artifact `11874982631` has SHA-256 `4a8edf845a46028d4684790e391c945f989c693dabb62232a1ef16ebc094ea22`.
The production store hash is `36ec5fff36c9b671f1ab1155cca1d0063388489cb973c31f9d9ffea1c90ca630`.
The regression hash is `e48ee04692a4fc364f2662d7a8847d3ea3c2fabc62a146890c855ed218ade98e`.
Both source pins match the committed files. The archived command contains the focused selector and scoped environment flag.
Read-only local xcresult inspection agrees with the hosted summaries and test tree.

The repair shares directory validation, protection, and backup exclusion between both writers.
Validated existing completion reads also repair the flag, without changing their saved contents.
Preparation failures retain fixed diagnostic fields, a generated request ID, and a numeric error code.
No raw paths, profile identifiers, saved data, or error descriptions belong in those logs.

The production delta adds 24 lines and removes six. The regression is unchanged from the accepted baseline.
Independent source review found no blockers. Fixed hosted execution and runtime log evidence remain pending.
