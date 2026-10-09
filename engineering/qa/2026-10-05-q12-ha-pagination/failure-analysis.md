# Q12 / CP02: bounded Home Assistant continuation

## Status and exact source boundary

This is test-first source preparation for an approved optional P2 opportunity.
No Q12 compiler, formatter, Python/HA runtime, browser or public peer has run.
No production behavior was changed. No Server defect is established.
Both current main refs were read through GitHub before authoring this checkpoint:
- Kinosail: e85544ebce14aab0454ea11d1fb01415dfbedd4f.
- Home Assistant adapter: 4c6fb5291429cbf9cbe25a87096ee5337f4fa134.
The adapter still equals its audit anchor. All four audited source hashes match.
source-inventory.json records 48 byte-verified inputs and scoped AGENTS absences.
Core types were read at HA 2026.9.0, the HACS minimum, not an installed runtime.

## Existing public contracts

GET /api/v1/home-assistant/library returns items, view, total, offset and limit.
The Player Browse callback uses canonical catalog browse and ClientItem projection.
ParseBrowse bounds each limit to 1..200, offset to 0..1,000,000 and q to 512 UTF-8
bytes; duplicate and unsupported query fields fail before loading the Library.
Filtering and visibility precede total and page selection.
An offset beyond the current total returns an empty page without clamping offset.
Default order is title; search ranking precedes title/ID tie-breaking.

The adapter's library(query) always requests limit=200 and returns only items.
Its media source calls that operation once for browse and once for search.
Search already exists and server directories retain can_search=True.
These facts identify missing continuation in source, not runtime reproduction.

Home Assistant grants require a current Owner. HA control accepts an Owner or an
API key with home-assistant scope. Do not synthesize a restricted Viewer HA grant.
An ordinary Viewer must be rejected by the HA route without item/total leakage.
That Viewer's public Library pagination is a separate visibility control.
Do not relax pairing, TLS verification, redirects, authorization or API scopes.

HA SearchMedia has result/version but no extra total or page fields.
Use existing nonplayable BrowseMediaSource entries and not_shown for presentation.
Do not add unnecessary Go Server fields or rewrite the existing Search contract.

## Test-first operation and navigation proposal

Preserve library(query)'s list return and all existing exact delegation tests.
Prepare additive library_page(query, *, offset=0, limit=200) returning bounded
items, total, offset and limit. Omit offset=0 on the wire to retain the existing
first-page request shape. Every new request keeps limit<=200 and offset<=1,000,000.
Reject bool, negative/out-of-bound numeric inputs before network activity.
Keep query scope and the Server's existing default order on every page.

Validate page metadata as non-boolean integers and match the requested page.
A populated page must match the current snapshot's count and unique safe IDs.
An empty page at or past the new total is legitimate after Library shrinkage.
Malformed, missing, duplicated or inconsistent metadata must not create a Next
directory. Keep the existing 2 MiB response cap and 10 s request deadline.

A proposed stateless identifier is entry/@page/<base64url JSON>. Its closed
payload is version=1, entry, query and offset. Bind entry to its configured runtime.
Bound encoded length to 2048 and decoded bytes to 1536; validate canonical encoding,
duplicate JSON fields, exact keys, query bytes and offset before making a request.
Reserve @page so it cannot be accepted by the existing playable ID grammar.
Unknown/foreign entries, wrong domains, malformed and out-of-bound pages must
produce fixed BrowseError/Unresolvable outcomes and no library/playback requests.
No query history, full-Library cache, persistent continuation state or auto-fetch.
The grammar is a proposed contract; its implementation needs source review.

Use a nonplayable count entry such as Showing 1-200 of 201 only when pagination
or a continuation context makes it relevant; preserve short-Library titles/items.
Use one bounded Next page directory with can_play=False, can_expand=True.
Search appends the same directory and preserves the original query.
At the final or empty page, show truthful counts and no Next.
Changing Library contents are evaluated per request; offsets are not snapshot
cursors. Never claim no gaps/duplicates across concurrent Library changes.
Do not silently fetch additional pages to hide that limitation.

## Failure modes and meaningful controls

1. Current browse/search can omit item 201 without a displayed count or Next.
2. A Next action could drop q, restart offset, change Server or exceed 200 items.
3. Count/limit/offset metadata could be absent, bool, negative or inconsistent.
4. Invalid IDs or duplicate current-page IDs could become playable children.
5. A paging ID could be resolved as media or trigger unwanted playback.
6. Forged/foreign/nested page identifiers could reach another runtime.
7. An offset could overflow, query bytes could exceed the Server bound, or invalid
   base64/JSON could allocate unbounded memory before rejection.
8. A changed Library can shrink to an empty final page; no endless Next loop.
9. Empty/error results could hide Search or report stale displayed/total counts.
10. A restricted Viewer could receive Owner-only items or totals.
11. Pagination could bypass the existing bearer/TLS/no-redirect/response bounds.
12. A fixture could replace Server behavior and falsely claim public proof.

## Proposed owned test files and scope

Kinosail apps/player/internal/server/home_assistant_pagination_fixture_test.go:
construct the real unmodified Player Server with only disposable fictional files,
real setup/MFA, Owner-scoped HA pairing, native loopback HTTP and bounded reads.
Reuse existing APIFixture/testTOTP and public profile/login helpers.
Use 201 visible and seven separate restricted-folder fixture items.
Fake media contents prove index/browse metadata, not playable-media decoding.

Kinosail apps/player/internal/server/home_assistant_pagination_contract_test.go:
prove actual HA 200+8 pages, query-selected 200+1 pages, unique canonical IDs/order,
empty final page, invalid browse rejection and unchanged HA authority.
Separately prove restricted Viewer's Library 200+1 pages and no private totals.
This is an actual Server/public API control when run, not HA browser UI proof.

Adjacent HA tests/test_api_pagination.py and tests/test_media_source_pagination.py:
use established Response/Session and real HA fixture/type conventions to test the
additive page contract and existing browse/search/resolve interface.
Mock HTTP/client cases are isolated. State their live-network/UI gap explicitly.
They can reproduce feature absence at the public adapter interface when run;
a missing new method is not evidence of a Go Server defect.

tests/test_pagination_public.py is not allocated. A future cross-repository live
bridge requires a reviewed fixed Server source, loopback origin, grant handoff,
fixture cap, lifecycle budget and safe reporter before authoring or dispatch.
A real HA frontend/accessibility render remains a separate delivery requirement.

## Execution and privacy prerequisites

Root owns repository integration, refs, workflows, bounds and hosted scheduling.
Before actual proof, review/format the exact Go pair, collect the exact HA tests
with Python 3.14.7 and declared plugin dependencies, and verify the pinned Server.
Run public Server controls twice on fresh fixtures; then isolated adapter REDs.
Count fixture/setup/collection/import failures as incomplete, never product RED.
Retain existing API/media-source, transport/security and normal required gates.

Proposed receipts keep fixed case IDs, status/phase enums, counts, booleans, source
and fixture digests, safe bounds and executed counts only. No tokens, real paths,
origin URLs, query strings, raw bodies, error messages, traces or user media.
Use the existing four-JSON receipt/results/source/artifact allowlist if root adds
a fixed hosted bridge. Bind both repository revisions and all selected inputs.
Unknown errors, skipped/retried cases or unsettled owned processes stay incomplete.

## Remaining boundaries

All proposed Q12 cases are unexecuted. No current UI, actual HA install, cross-repo
network bridge, compiler, formatter, collection, acceptance RED/GREEN, required
gate, merge or deployment is claimed. No product method or page parser exists yet.

## Historical first test-source checkpoint

Four new test sources are staged as unreferenced GitHub blobs, with GET byte
verification and <=300 lines each. The Go pair has three registered tests;
the HA pair has seven API and eight media-source test methods.
Parameter expressions imply 39 API and 21 media-source cases if collected;
collection has not run. Do not report those inferred counts as executed.
The native Go fixtures configure the two actual Library roots explicitly,
matching the existing visibility fixture. The Viewer only receives Visible.
POST /api/v1/session is source-verified HTTP201, not an assumed HTTP200.
These were source-only prerequisite corrections before any execution.

The initial three QA blobs and earlier unreferenced fixture candidate are retained.
No prior source-only checkpoint is upgraded into runtime evidence.
The final source inventory separates selected current inputs from new candidates.
Source cap/whitespace and JSON equivalence checks ran in RAM; syntax/format/compile,
HA import/collection, all three Go tests and all adapter cases remain unrun.

## Reviewed-contract revision before product work

The two new HA files now require all200 playable first-page IDs, exact count,
titles and response order alongside count/Next for both browse and Search.
The initial strict API page test also compares all200 item records, not just length.
Earlier summary-only tests could pass after dropping initial items; these new
expectations expose that false acceptance before any product change.

Initial browse/Search and every new query retain library(query) delegation.
A documented list-compatible result may carry optional immutable .page metadata
owned by that response. Valid complete page metadata is attached only after the
strict page validator succeeds. Missing or invalid metadata remains a valid
legacy list/items-only response, unpaged and with no summary or Next.
Plain list clients naturally stay unpaged. There is no test/client-type detection
and no mutable client-level last-page state. library_page is additive and strict,
used only for continuation. Old items-only, wire-shape and delegation tests stay
byte-exact in their original files. The carrier proposal is not implemented.

A new actual async_search_media call starts a NEW query from each of two
can_search=True directories: a continuing page and a final page. It requires
the same configured entry, exact fresh results, offset reset0 (omitted on wire),
limit200, one additional request and zero calls to another configured entry.
Current rejection of slash identifiers is source feature absence, not a diagnosed
runtime regression. Never hide Search or silently remove these acceptance checks.

Identifier tests now deny padded and noncanonical valid Base64 encodings, bool
and float version values, and non-string queries before requests or playback.
A RAM-only equivalent confirms the noncanonical vector decodes to the same bytes
but re-encodes differently; this is vector preparation, not Python parser proof.
The offset-ceiling test returns200 items with one further result at total1000201:
show truthful1000001-1000200 counts, no unusable Next above1000000, and retain
Search so the user can refine. No auto-fetch or cursor semantics are introduced.

A lone-surrogate NEW query at initial/continuation locations must yield fixed
MediaSourceError("Kinosail search query is invalid") before transport.
This intentionally chooses the Core source error type for this new validation
boundary; existing paging BrowseError/Unresolvable translation stays intact.
Core2026.9.0 __init__, error and helper sources were read and added to the
selected inventory. The exported type exists; installed import/frontend handling
is unproved. Catching or leaking UnicodeEncodeError is not acceptable evidence.

Revised HA sources contain9 API and12 media-source methods, implying45 and34
cases from authored parameter vectors if collection succeeds. All actual counts
remain0. Mechanical insertion inverses restore the prior new HA sources exactly;
at that revision all old files and both Go fixture blobs remained unchanged. The first source-only
checkpoint and its aggregates are retained in history. Syntax/import/collection,
formatter, public Server, HA frontend, acceptance RED/GREEN and gates remain unrun.

## Narrow disposable-fixture prerequisite correction

Independent source review found settings.go initializes UpdateChecks=true.
Managed application construction schedules the update checker before setup can
disable it. Leaving updateChecks absent could issue an external GitHub request.
This is a source-supported fixture risk, not an executed Q12 product failure.

The source-prepared guard-first fixture requires a present false bool read from
its actual temporary settings before Server.New. That unexecuted stage retained
the old JSON omission, so the guard would fail rather than admit public evidence.
The following candidate adds exactly updateChecks:false to its disposable JSON.
No production/real-user settings were modified. The guarded fixture still uses
real setup/MFA/pairing and the original three public Go contract tests.
Blank AuthURL already prevents discovery registration; no discovery code changed.

Its native peer client now owns a Proxy:nil HTTP transport with an exact
declared-loopback scheme/host guard and no URL user info; redirects remain denied.
A foreign request is rejected with a fixed error, never exported raw details.
This controls only fixture-owned public requests. It does not instrument every
possible Server background request or prove general egress isolation.
The update checker configuration/source guard and fresh hosted network boundary
must remain prerequisites. Formatter/compile/lifecycle/native peer guards unrun.

The inventory adds six reviewed settings/construction/checker source inputs,
now47 selected inputs; historical38/41 aggregates and guard-first blob are retained.
Removing documented transport/readback insertions restores the original fixture;
removing only updateChecks:false restores the immediately prior guard-first bytes.
No valuable test or production contract was deleted or weakened.

## Factory extraction and actual HA presentation gate

Independent source review inferred cyclop complexity12 and about46 nested
statements in the earlier factory against unchanged Player limits10 and40;
funlen also limits functions to60 lines. No formatter or actual linter ran.
The fictional population loop now lives in q12PopulateMedia, called at its
original position. Root open and deferred close remain in the factory, preserving
close timing. Peer/client statements live in q12PaginationPeer, called at their
original position after Viewer enrollment. All guards, permissions, transport
settings and cleanup registrations/order are byte-exact.
Removing both helper wrappers and substituting their bodies mechanically restores
c804188d completely. Inferred factory/population/peer complexities are4/5/5;
the inferred factory has32 nested statements. Canonical formatted line counts
and actual funlen/cyclop acceptance remain pending. No nolint or threshold change.
The peer-only extraction blob is retained as an unexecuted historical stage.

Core2026.9.0 helper.py can apply content_filter to actual browse children,
retaining expandable rows or those accepted by the player filter. The proposed
nonexpandable/nonplayable summary can therefore be filtered out. Direct-method
adapter mocks alone do not prove displayed/total visibility in that real path.
Actual Core helper/filter and HA frontend acceptance must demonstrate the count
and Next presentation before Q12 completion; a source-supported delivery risk
is not a diagnosed runtime failure. This gate remains distinct from the real
Server API controls and from isolated adapter direct-method tests.

The unchanged Player .golangci.yml was also GET byte-verified and pinned; the
selected inventory now has48 inputs and retains historical38/41/47 aggregates.
