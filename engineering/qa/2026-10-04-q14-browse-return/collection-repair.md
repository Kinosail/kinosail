# Q14 source-only collection repair

After the explicit user restart, the parent authorized source preparation only.
The checkpoint remains in the same leased `codex/qol-q14-browse-return-tests`
checkout. Production and CI remain unchanged. No Go, lint, test collection,
browser, UI, media, encoder, or native execution validates this repair yet.

Independent review found that the original `de2f03fa` supplemental slices put
worker-scoped `launchOptions` and `channel` inside `test.describe`. Pinned
Playwright 1.63.0 `lib/common/index.js` rejects this before any journey runs.
`lib/index.js` declares both fixtures worker scoped. The primary two-case slice
did not register those blocks and was not affected by that collection blocker.
This is a test harness defect; it is not evidence of a product failure.

The repair moves cold-native and BFCache launch settings to file scope:

- `browse-return.spec.ts`: primary (2), HTMX (1), Shows (2).
- `browse-return-cold.spec.ts`: cold native (2), live-search cold native (1).
- `browse-return-bfcache.spec.ts`: actual native BFCache admission control (1).

The opt-in Go owner maps its already validated mode to these fixed spec names.
`all` selects all three, retaining nine cases, one worker, and zero retries.
All nine journey bodies remain unchanged apart from relocating comments and
launch configuration. Helper, catalog, route, profile, focus, scroll, duplicate,
fresh-document, history-request, and native admission assertions remain intact.
Failed native BFCache admission remains a prerequisite failure, not product RED.

The original analysis, commands, preparation checks, context, and five-entry
manifest stay byte-preserved as the historical `de2f03fa` boundary. Their passed
checks do not validate the later split files or updated Go harness. Read
`collection-repair-context.json` for current source hashes and preserved-body
comparisons; read `collection-repair-commands.md` for the pending bounded runs.
The new runner writes only a unique task verification directory, verifies the
authorized fictional clip, preserves exit/report/log receipts, and terminates
only its newly created disposable process group. It has not executed.

Q09 remains frozen at `1b68c55b8fa176154f2a17052606f71867208308`, with its
original product and all 250 artifacts preserved. Root owns its integration.
Runtime RED, explicit production go-ahead, profile/error controls, required
consumer gates, merge, and ancestry remain pending.

MAIN: NO — source repair only; parent-scheduled runtime proof is pending.
