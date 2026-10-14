# Android backup exclusion policy

The app and Wear client intend to exclude their data from backups and device transfer.
Their rules currently exclude `root`, `device_root`, and `external` in both transfer sections.
Independent source review found six uncovered domains: files, databases, and preferences in credential-protected and device-protected storage.

Android scans each domain independently. The export traversal compares exclusions with exact canonical paths.
A root exclusion does not cover a later scan that starts inside a different domain.
The XML parser does not expand root exclusions into those domains.
See [Android 15 BackupAgent](https://raw.githubusercontent.com/aosp-mirror/platform_frameworks_base/android-15.0.0_r1/core/java/android/app/backup/BackupAgent.java)
and [FullBackup](https://raw.githubusercontent.com/aosp-mirror/platform_frameworks_base/android-15.0.0_r1/core/java/android/app/backup/FullBackup.java).

`allowBackup=false` remains useful, but some Android 12+ device transfers ignore it.
See the [Android backup guide](https://developer.android.com/identity/data/autobackup).
This is a configuration defect. No actual transfer or disclosure has been observed.

## Failure paths and verification plan

Before changing the rules, add a behavioral regression using each module's compiled resources.
Existing browser, caption, and Wear launch journeys do not invoke Android's backup agent.
The retained isolated test exercises Android's XML parser and `BackupAgent.onFullBackup` under Robolectric 4.17, SDK 35.
Only the native tar writer is replaced with a recorder of selected paths.

The test must distinguish these failure paths:

- Separate domain traversal can export private data despite root exclusions.
- Cloud and device-transfer sections can differ. Select the agent's destination explicitly for each check.
- Shared framework caches can reuse another destination or an earlier policy. Clear them between operations.
- Missing canaries or shared credential/device directories can produce a false pass. Verify nine existing files and distinct storage paths.
- A disabled traversal or broken recorder can produce a false pass. A no-rules control must select all nine canaries.
- Directory entries can inflate counts. Inspect only the seeded canary files.
- Restore filtering can reject a descendant after export. Inspect export selection rather than relying on a restore-only check.

The initial hosted run must retain its failures before the XML repair.
The corrected policy must select no canaries in either destination for both modules.
Hosted reports must retain the exact revision and test results.
This checks framework file selection. It does not establish OEM transport admission or a physical device transfer.
Local Gradle execution remains held.

## Initial setup failure

Run `38516297794` tests baseline revision `d8e5b7c1aeffe1a30a5c9a92a872ef431144db6a`.
All three cases in each module stop at the missing backup-rule resource guard.
No file-selection assertion executes. This is a rejected setup result, not policy reproduction.
Artifact `11871834716` has SHA-256 `b0fa3759fb2ace74022881acf071d5adaf1c5be2ff1076ac64e2d2acf47ec674`.

[Robolectric 4.17 uses the legacy PackageParser](https://github.com/robolectric/robolectric/blob/robolectric-4.17/shadows/framework/src/main/java/org/robolectric/shadows/ShadowPackageParser.java).
[Android 15's legacy parser](https://github.com/aosp-mirror/platform_frameworks_base/blob/android-15.0.0_r1/core/java/android/content/pm/PackageParser.java) does not read `dataExtractionRules`.
The test now reads that resource reference directly from the compiled manifest.
It checks the package, disabled backup flag, and nonzero named resource before binding the missing field during each operation.
The framework still parses the real backup XML and selects files. The original field is restored afterward.
Both production XML files remain unchanged while the corrected baseline runs.

The next baseline, `70a503a689ed2159f63f595f554e71c225b1f4db`, passes those manifest guards.
It then stops before file selection because the simulated user's credential-protected preferences are locked.
All six cases retain that setup failure in artifact `11872398231`.
Its SHA-256 is `6161ecfcee56f38b91906ae61b27c8c614ba137776dc4bf87bd864c693bd0599`.
The fixture now sets the current user's state to `STATE_RUNNING_UNLOCKED` before creating canaries.
This uses [Robolectric's per-user state API](https://github.com/robolectric/robolectric/blob/robolectric-4.17/shadows/framework/src/main/java/org/robolectric/shadows/ShadowUserManager.java).
The separate no-argument unlocked flag does not configure the per-user check used by credential storage.
Preference creation still uses a real committed write. The nine-file control and production XML remain unchanged.
