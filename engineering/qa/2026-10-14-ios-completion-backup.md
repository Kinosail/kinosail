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
