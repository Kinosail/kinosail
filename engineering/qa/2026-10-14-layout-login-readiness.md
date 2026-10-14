# Layout login destination readiness

## Contract and failure cases

The layout login helper can follow the optional **Not now** link after signing in.
It must finish loading that destination before the caller starts its next navigation.
An indefinitely pending destination must reject the helper without requesting Library.

The populated-instance helper already has isolated HTTP controls for this contract.
The layout helper has a separate implementation and is not exercised by those controls.
The original click had no explicit destination-load barrier.
The hosted baseline reproduced early completion and premature Library navigation.

The existing fixture now runs both helpers through the same real form and redirect chain.
A held Home script separates navigation commitment from destination load.
The delayed control requires no early return or Library request, then exactly one Library navigation after release.
The blocked control requires the existing three-second navigation timeout and no Library request.
Both controls still require exactly one login submission and one Home request.

The important failures are an early successful return, premature Library navigation, and an ignored destination timeout.
The existing populated Server journeys cannot hold this particular post-login resource deterministically.
These tests are isolated HTTP fixtures, not populated-server E2E acceptance.
No application response or production login code changes in this baseline.

## Evidence

The test retains bounded request counts and helper completion observations before fixture cleanup, including on failure.
The observations omit credentials, URLs, and raw errors.
The browser report and source manifest bind them to the hosted revision and command.
Local browser execution remains held.
The existing browser retries and deadlines have not changed.

### Failing hosted baseline

Run `38596823703` tested baseline `8bd2898b752cbe26441f3886f80821923617c517` through merge revision `58020d27b69c44d351e299723adf4445d0484f07`.
Their Git trees match.
Artifact `11912449326` has SHA-256 `edcf6f631f1bca8503895878c6530332748402fe8df7022dae6f5dd0cfe76de9`.
Its clean revision receipt, ten source pins, 386 child hashes, and 387 checksum entries verify.

Chromium `153.0.8010.12` passed both populated-instance helper controls without retries.
Both layout helper controls failed on their original attempt and the configured retry.
All four failed attempts recorded one login submission, one Home request, and one premature Library request.
The layout helper returned with Home still `interactive` and its held script uninitialized.
These observations were captured before releasing the fixture during cleanup.
The aggregate report has 454 passing executions, two failed cases, 63 skipped executions, and no global errors.
This failed run does not establish populated settings acceptance; that later suite was not reached.

### Repair and verified delivery

The layout helper now waits for the **Not now** destination's `load` event alongside the click.
It uses the destination URL from that link, matching the existing populated-instance helper.
The blocked destination still uses the unchanged navigation timeout and must prevent Library navigation.
No application code, retries, deadlines, or assertions change in this repair.
At `8f63db4278cad868f1ceef9025b0cbd3fbd285b0`, all four destination controls pass in each browser without retries.
Each retained artifact verifies against its hosted checksum, clean revision, and nine measurement-script source pins.

| Browser | Artifact | SHA-256 | Broader layout result |
| --- | --- | --- | --- |
| Chromium | `11913939879` | `f374e36f9027a2fcb0e9047d6585b02a8f3f34d34d64e354e48b89929851ae21` | Player and Subtitles pass |
| Firefox | `11914131684` | `f64102139c784fd12c3ac9df2b00c23ff135375bb6d423c5dbf999b32fe5c0a9` | Player passes; later Subtitles navigation fails |
| WebKit | `11913539582` | `4e5a3eb3ec6308c717c7154ab77132aa89e0430fdecc31162a64136db2a89cf7` | Player and Subtitles pass |

The Firefox failure occurs in the theater journey before its assertions, at `layout-stability-flows.mjs:90`.
The navigation times out after 30,005 milliseconds, although the Server returns HTTP 200 and the screenshot shows rendered playback.
The trace records 31 requests without network failures, while the driver reports a blank URL.
Its SHA-256 is `95c3590881986c268a3b53687cb19ed812d54b388d4f48d8ef4400509532a6d2`.
The relevant Subtitles, shared packages, measurement scripts, and browser lockfile match the previous green layout revision `3b3e2ecd`.
[Playwright issue #42183](https://github.com/microsoft/playwright/issues/42183) reports similar Firefox navigation symptoms.
This trace lacks the document and lifecycle probes needed to establish the same cause.
One unchanged replay of the failed Firefox job passes; the original failure remains retained and unexplained.
Artifact `11916480592` has SHA-256 `5b98c95d0501664c761fcb76b1402469ca81f64539b97d5c33b3df013074391e`.
Its clean revision and nine script source pins verify.
All 29 Player flows, 33 Subtitles flows, and 43 isolated controls pass without retries.

Required CI run `38601559279` passes all four protected gates.
The tested merge revision `939e01e9aa509611cb7e4eca862b68e7e0a15893` has the same tree as the reviewed head.
Player artifact `11916360374` verifies 536 passing executions, 466 distinct passing tests, and 63 skipped executions.
Its SHA-256 is `1250f92f21b6295cb1493c2f975cf527bc5f2d8d19d38eed7ea29bc412dbdcbc`.
Subtitles artifact `11915375980` verifies 38 passing tests.
Its SHA-256 is `de7eb45f6bffd08bd0f46a6767d0edcb9b3962b939e3b12bcaaf8e50d0d2f7a3`.
Both artifacts have zero retries, unexpected failures, or global errors.
Their ten source pins and all 504 combined child hashes verify.
These totals include isolated fixtures and do not represent only populated-server journeys.

PR #604 merged at `be52f79d1f4c81a24954df15b7ee271b7799a1b5`.
A fetched ancestry check confirms that remote main includes the reviewed commit.
Exact-main publication remains pending.

## Hosted coverage routing

The existing layout workflow now selects the login readiness controls in Chromium, Firefox, and WebKit.
Changes to either the layout login helper or its readiness tests trigger that workflow.
An extended CI contract fails before this routing change, then passes after it.
Browser execution alone cannot prove that an omitted test file or path filter will run.
The original failure log has SHA-256 `abfd1abfda087f2ed2a34355c2b98bd092ee747d2f9feb803457f4605f8b11e7`.

The local CI contract suite runs 192 tests with five skips for hosted scanner integration.
Workflow validation, pinned Actionlint 1.17.0, source limits, and whitespace checks pass.
The suite log has SHA-256 `9f24f50ec24b403add47dc400a31932d5ef0241d640d11bff9c94ae4722372ea`.
This source validation does not establish browser acceptance.
