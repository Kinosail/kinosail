# Android backup exclusion policy

The app and Wear client intend to exclude their data from backups and device transfer.
Their previous rules excluded only `root`, `device_root`, and `external` in both transfer sections.
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
The separate no-argument unlocked flag does not configure the handle-based per-user check.
Preference creation still uses a real committed write. The nine-file control and production XML remain unchanged.

That attempt, `d253d92912e950754696d0c5fbcb31bde0250b40`, also stops at the locked-preference guard in all six cases.
Artifact `11872643218` has SHA-256 `ea365376ae27343e22acf0fe6991f3482e78a41b91a622e1dec5ffde13c8555e`.
[ContextImpl](https://github.com/aosp-mirror/platform_frameworks_base/blob/android-15.0.0_r1/core/java/android/app/ContextImpl.java) calls the integer-user overload of `isUserUnlockingOrUnlocked`.
Robolectric shadows the handle overload, so the attempted state setup does not cover this call.
This remains a rejected setup result, with no file-selection execution.

The final fixture creates all nine synthetic files directly at paths supplied by the framework, including `getSharedPreferencesPath`.
Backup selection inspects paths and file metadata; it does not parse database or preference contents.
The fixture therefore needs no preference API writes or user-lock shadow.
It still requires each file to exist, distinct storage paths, six selected domains before repair, and all nine domains in each control.
The test makes no claim about preference creation, database validity, or storage-unlock behavior.

## Reproduced policy failure and repair

Baseline `1c8d0adaac59e708f79465ea74a5efac525e315f` reaches file selection in both modules.
Run `38519156011`, job `115705191648`, records exactly two policy failures and one passing control per module.
Both cloud and device-transfer checks select `file`, `database`, `sharedpref`, `device_file`, `device_database`, and `device_sharedpref`.
Both destinations select all nine files when the rules are removed. The other 166 unit cases pass.

Artifact `11873770394` has SHA-256 `6b81c2cdf32d98de4b4bf5ab223c683d967f3440db472b32aa2ae082453e3a2c`.
The app report SHA-256 is `1d19c4cf310cb66083d1ca7d6430ef9876dc2cd1768ea443bb43f53dfd4b9638`.
The Wear report SHA-256 is `8634d46a928bd60fc5eaf935de429ceae0e0fcb521069dafb88bbbd655c69d4d`.
This is accepted framework-selection reproduction, distinct from the earlier setup failures.

The repair adds explicit `.` exclusions for those six domains to both sections in both XML files.
The existing root and external exclusions and disabled-backup manifest flags remain intact.
The regression is unchanged from the accepted baseline.
Fixed revision `11a9a696a62edfb5a882b6c9bb838f7b5aee17cd` passes all six cases in run `38520041411`, job `115708078212`.
Each policy selects zero canaries. Each no-rules control still selects all nine, and the other 166 unit cases pass.
Artifact `11874116320` has SHA-256 `b07af061cae89abe271de70fd96d6acd2701db11a8217d79686e788cedd67c59`.
The actual checkout is `2d0bba2abf7077ba57deb28096b8556696bb764a`.
Its tree equals the branch tree, `2e370021b0ac78f6d5258a96ddb8174412dac637`.
The lint artifact `11873911599` has SHA-256 `e20363cbcc017b0a800b96d158e31ee8b0cfab18087c848d55e05ce4c9f03ed9`.
It contains zero errors, 26 warnings, and one hint.
No actual OEM transfer or disclosure is established by this test.
