# Q14 direct CLI prerequisite preparation

This is source-only preparation following the collection timeout. No direct
CLI, collection, Go, lint, browser, UI, or media command has run in this phase.
The timeout receipt remains intact and establishes zero executed Q14 cases.

## Executable and dependency observations

The current shell resolves pnpm to `/opt/homebrew/bin/pnpm`, a symlink to the
Homebrew 12.4.2 Rust binary. Player E2E's `packageManager` pins `pnpm@11.22.0`.
The installed `.brew/pnpm.rb` formula says subsequent invocations resolve that
pin against the registry. This supports a registry-resolution startup timeout
hypothesis. The actual 15-second failure cause remains unconfirmed: no launcher
trace or network result was captured. Do not label it a browser collection bug
or a proven registry outage.

The existing 707-byte `@playwright/test/cli.js` directly requires
`playwright/lib/program` and parses arguments. Its package and linked
playwright/playwright-core packages are 1.63.0. The direct CLI checksum is
`79e23e6a249176295b8490567daa7717448a75866d6ea6f6b296ff3d23305c69`.
The core bundle matches the earlier `549070af...` digest. The dependency tree
currently resolves under `/Users/mikeo/Documents/GitHub/kinosail`; that path
differs from historical preparation. Record the actual resolved path and hashes
at each execution. The existing bin shim has unrelated absolute NODE_PATH
entries, so the direct CLI also avoids reliance on that shim. No shared link,
global dependency, package-manager configuration, credential, or scanner changes
are authorized or performed.

## Narrow source repair and next scheduled command

The test-only Go owner now invokes `node node_modules/@playwright/test/cli.js`
from its E2E directory with the same validated spec selection, one worker,
zero retries, and real disposable Server. This avoids package-manager startup.
It does not prove that direct Node collection succeeds. The existing two-run
bounded runner still governs the later public primary RED and fixture checksum.

After the next explicit parent grant, run this collection-only command from
`apps/player/e2e`, with a 15-second external process-group bound:

```sh
PLAYWRIGHT_CHANNEL="" KINOSAIL_BROWSER_PROJECT=chromium \
KINOSAIL_BROWSE_RETURN_URL=http://127.0.0.1:9 KINOSAIL_BROWSE_RETURN_CASES=all \
/opt/homebrew/bin/node node_modules/@playwright/test/cli.js test \
  browse-return.spec.ts browse-return-cold.spec.ts browse-return-bfcache.spec.ts \
  --list --workers=1 --project=chromium
```

This uses an unreachable origin only to register fixture-dependent cases. It
performs no HTTP or browser journey and must be described as collection only.
Expected discovery is nine cases. Stop on prerequisite failure; preserve exit,
stdout/stderr, exact executable/dependency/source hashes and process state.
Do not silently extend the bound or substitute synthetic application proof.

If collection completes, obtain the parent's grant before the two actual Go
primary runs. Each remains Go70s/external75s with five seconds solely for
owned-process shutdown. Root schedules native control first. There is no active
Q14 slot, process, or runtime permission in this preparation phase.

## Assertion preservation and evidence boundaries

The main spec's EOF repair removed one final LF for whitespace. All prior
journey assertions remain. Independent review requested one additional BFCache
peer-count check after `assertReturn` has settled focus/scroll. That assertion
is now prepared while retaining the earlier native-admission and immediate
peer-count checks. It closes the delayed-continuation coverage gap; it has not
executed and creates no native cache proof.

Historical `de2f03fa`, collection-repair, and timeout manifests stay unchanged.
`direct-cli-preparation-context.json` pins the new current inputs and the
executable/link metadata. Source-cap, whitespace, and lease checks alone are
permitted now; prior syntax/gofmt observations do not validate this later CLI
and additional assertion delta. Root owns independent review, later integration
and required protected checks. Production still requires accepted public RED
and an explicit implementation go-ahead.

MAIN: NO — direct CLI prerequisite and public RED are pending.
