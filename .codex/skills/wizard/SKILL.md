---
name: wizard
description: Create an interactive shell wizard for setup, credentials, dashboard steps, or a cutover that requires human actions.
---

# Wizard

Use [template.sh](template.sh) for its progress, hidden input, environment updates, and confirmation helpers. Do not edit the library above its `STAGES` marker.

First inspect configuration examples, documented variable names, workflow references, and current setup instructions. Do not read secret values from real environment files unless the task requires them. Identify each human-only action, the value it produces, where that value belongs, and whether it is secret.

Create one focused stage per dependency-ordered action. Open the relevant URL before asking for a value. Use hidden input for secrets, persist only intended values, and write only CI variables referenced by the repository. Confirm immediately before irreversible operations.

Run `bash -n`, run `shellcheck` when available, make the file executable, and statically trace every captured value. Do not run a credential or dashboard wizard end to end on the user's behalf.

Keep a repeatable wizard only when the user wants it maintained; otherwise place it in a temporary or clearly disposable location.
