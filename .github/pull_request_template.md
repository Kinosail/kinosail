## Problem and result

<!-- Describe the concrete problem and the resulting behavior. Name the affected apps. -->

## Verification

<!-- List exact commands/results, manual observations, and remaining boundaries. -->
<!-- If .gates-disabled exists, record disabled checks as not run; do not claim they passed. -->

## Documentation and compatibility

<!-- Link changed setup/API/configuration/recovery docs, or state why no docs change is needed. -->
<!-- Describe migration, state, permissions, or client compatibility impact where relevant. -->

## Architecture

- [ ] Changed capabilities share application validation across web and versioned API adapters, or this is a documentation-only change.
- [ ] Each app retains its independent Server container; public Player/Subtitles HTTPS uses the separate restricted gateway where applicable.
- [ ] No secrets, private media, account data, or unrelated task changes are included.

## Contributor agreement

- [ ] I have read and agree to the [Kinosail Individual Contributor License Agreement](https://github.com/Kinosail/kinosail/blob/main/apps/player/CLA.md).
- [ ] I am entitled to submit this contribution, including any required employer authorization or [Corporate CLA](https://github.com/Kinosail/kinosail/blob/main/apps/player/CCLA.md).
