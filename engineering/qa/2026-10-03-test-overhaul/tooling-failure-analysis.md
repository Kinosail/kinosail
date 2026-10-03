# Tooling audit failure analysis before assertion repairs

Base: `876b771a7dd0aef5e65957fcee87add0312535e8`.

## Apple build argument contract

The public command is `build-apple.sh ios|tvos|watchos [simulator|device]`.
The implementation accepts an omitted or empty second argument as `simulator`.
The native owner confirms supported targets before this test is repaired.
These are the failure paths this isolated check can observe:

- Missing target or excess arguments must fail before Git or Xcode effects.
- Unknown, oversized, or newline-bearing targets must fail before those effects.
- Unsupported build modes must fail before those effects.
- A side-effect marker from one parameter must not contaminate later results.
- Supported watchOS and default simulator inputs are valid; classifying them as
  negative cases produces a false failure without finding a product defect.

The baseline fails on the two valid inputs. Keep the negative contract and
remove those stale cases. Reset the fixture marker before each remaining case.
No new isolated test declaration or production behavior is added.

## E2E evidence upload boundary

The workflow will run `e2e-artifact.py` around the existing real container
launcher. Browser output is nested under `browser-results` within an artifact
root that the upload step retains on both success and failure.
The existing isolated check exercises the actual launcher's environment
assignment with a `pnpm` stub. It must remain labeled fixture evidence.
It can detect these failures:

- A launcher override writes results outside the uploaded artifact root.
- App or browser substitutions resolve to a different upload directory.
- Chromium, Firefox, or WebKit failure traces are omitted from that directory.
- A workflow rename or scalar upload path breaks a parser tied to the former
  multiline upload syntax, even while the release boundary remains correct.

Adapt the existing parser to the reviewed new step and scalar artifact path.
Keep every app/browser reachability assertion. This is a CI boundary exception:
a passing product journey cannot prove that its failure trace will be uploaded.

## High-confidence duplicate removals

- Each native build harness validates the installation contract extracted from
  the built host archive. Its production packager already validates the source
  copy. Running `test-native-contract.sh` after the packaged validation repeats
  the same assertion list without introducing a different failure boundary.
  Retain the packaged check and production guard; remove the redundant wrapper.
- The lint test's self-cycle bundle fixture repeats the canonical bundle
  resolver's cyclic composition test. Both reach `resolve()`'s visiting-set
  rejection. Keep the canonical resolver test and the lint-specific unsupported
  append case.
- Exact asset-order and bundle-name assertions in the lint test restate current
  declarations. The same test lints every actual combined scope and injects an
  unresolved global negative control. Keep those executable assertions, which
  independently detect invalid combined scopes after harmless asset refactors.

No removal is claimed to have equivalent product E2E coverage. These are
duplicate tooling assertions whose stronger executable owner checks remain.
