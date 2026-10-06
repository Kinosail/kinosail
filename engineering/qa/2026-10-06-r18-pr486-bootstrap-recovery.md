# R18 bootstrap recovery

This isolated candidate starts from verified main
`23d91d2cbb0fc7bdefd9d4d925d5661ba812be6d`. It preserves PR486's original
contracts and private evidence without changing its branch.

Hosted run `37515745539`, at original head
`5cba330b73efb0ec90a4a8b2f8f6b2163b30920f`, acquired its runner and passed
thirteen formatter controls. Seven bootstrap errors occurred before the public
proof: Python 3.12 pathlib lazily imported the standard-library ntpath module
inside the closed definition import guard.

An explicit regression was added before repair. It fails on the original loader.
The repair preloads and admits only ntpath; unknown modules and relative imports
remain rejected. All29 bootstrap controls, all50 document source controls and
all9 workflow controls pass. The fixed definition-only wrapper also admits
exactly19 controls with no failures, skips, formatter execution or Go execution.

The two Go contracts remain byte-identical to the original draft. An installed
canonical formatter proposes a whitespace correction in the helper; its output
is preserved privately for comparison with hosted source-format evidence. No
formatter proposal is automatically adopted. The unchanged official architecture
renderer retains current-main metadata and adds these two external test files.

The four public document-claim tests describe unfinished backend behavior.
Source controls and formatting alone cannot establish that behavior or R18
completion. Required checks, actual public product tests and claim-route behavior
remain separate delivery gates. No production source, scanner policy, required
check policy, external Home Assistant repository or deployment changes occur.
