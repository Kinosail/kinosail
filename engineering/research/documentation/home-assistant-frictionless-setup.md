# Frictionless Home Assistant setup

Research date: 2026-08-27

## Decision

Replace the primary copy-and-type pairing flow with this flow:

1. Kinosail advertises `_kinosail._tcp.local.` only while Home Assistant access is enabled.
2. Home Assistant discovers the server and shows one confirmation dialog.
3. The user selects **Connect**.
4. Home Assistant opens a Kinosail approval page in a new window.
5. The signed-in Kinosail Owner selects **Approve**.
6. Kinosail returns a short-lived authorization code to Home Assistant.
7. Home Assistant exchanges it with Proof Key for Code Exchange (PKCE), stores the scoped grant, and closes the window.

Keep the current eight-digit pairing code as a manual fallback. Do not make it the default path.

Home Assistant calls the popup pattern an external step. Its frontend opens the external URL with `window.open`. The callback advances the configuration flow and returns `window.close()` ([external-step documentation](https://developers.home-assistant.io/docs/data_entry_flow_index/#external-step--external-step-done), [frontend implementation](https://github.com/home-assistant/frontend/blob/dev/src/dialogs/config-flow/step-flow-external.ts)).

## Why this is the best fit

The result is a two-action connection after installation:

- Home Assistant: **Configure Kinosail**.
- Kinosail: **Approve**.

The user does not copy an address, password, token, or pairing code. Home Assistant never receives the Owner password.

Home Assistant supports Zeroconf discovery through the integration manifest and routes discoveries into `async_step_zeroconf`. Discovery must identify a stable server and must still ask the user to confirm ([config-flow discovery](https://developers.home-assistant.io/docs/core/integration/config_flow/#discovery-steps), [Zeroconf manifest](https://developers.home-assistant.io/docs/creating_integration_manifest/#zeroconf)).

Use discovery only to locate Kinosail. Do not treat multicast Domain Name System (mDNS) data as authenticated identity.

## Authorization design

Use Authorization Code with PKCE S256 for the preferred flow. Home Assistant provides `LocalOAuth2ImplementationWithPkce`, including the verifier and challenge exchange ([application credentials and PKCE](https://developers.home-assistant.io/docs/core/platform/application_credentials/)).

Kinosail must:

- require an authenticated Owner on the approval page;
- show the Home Assistant client name and requested permissions;
- provide explicit **Approve** and **Deny** actions;
- bind the code to one transaction, client, redirect URI, and PKCE challenge;
- match the redirect URI exactly;
- expire the code quickly and permit one exchange;
- issue only the existing narrow Home Assistant grant;
- keep access and refresh tokens out of URLs;
- rotate refresh tokens if refresh tokens are added; and
- revoke the grant when the Kinosail setting is disabled.

These requirements follow current OAuth security guidance ([RFC 7636](https://www.rfc-editor.org/rfc/rfc7636), [RFC 9700](https://www.rfc-editor.org/rfc/rfc9700)).

Do not require each user to create a client ID and secret. Home Assistant's standard Application Credentials flow supports that model, but it adds setup work. Cloud Account Linking is intended to remove that work for cloud providers. Kinosail should remain local and use a fixed public client or a focused custom external-step implementation.

## Kinosail entry point

Add an **Add to Home Assistant** button to the enabled Settings and wizard states. Use:

`https://my.home-assistant.io/redirect/config_flow_start/?domain=kinosail`

My Home Assistant selects the user's saved instance without sending its URL to the redirect service ([My Home Assistant](https://www.home-assistant.io/integrations/my/)). The redirect accepts only the integration domain. It cannot prefill a server address or secret, so mDNS discovery remains necessary ([redirect catalog](https://github.com/home-assistant/my.home-assistant.io/blob/main/redirect.json)).

The link does not install a custom integration. It starts the flow only after the Kinosail integration is installed.

## Installation friction

The popup solves connection friction, not installation friction.

For the complete experience:

1. Publish a versioned GitHub release for the Home Assistant component.
2. Add the required HACS and Hassfest validation.
3. Submit the repository for the default Home Assistant Community Store (HACS) list.
4. Later, pursue Home Assistant Core inclusion when Kinosail meets its maturity requirements.

Until HACS accepts the repository, users must add it as a custom repository. HACS documents that as a manual URL and repository-type step ([custom repositories](https://www.hacs.xyz/docs/faq/custom_repositories/), [default listing requirements](https://www.hacs.xyz/docs/publish/include/)).

## Fallbacks

Keep these fallbacks in this order:

1. Manual server selection after failed mDNS discovery.
2. External approval after manual server selection.
3. Existing eight-digit pairing code when the browser cannot complete the external step.

An RFC 8628 device flow can support cross-device approval, but Home Assistant has no documented generic helper for it. It adds polling and more server state. Do not add it until real use shows that the external-step fallback is insufficient.

## Required implementation slices

### Kinosail

- Advertise and withdraw `_kinosail._tcp.local.` with the existing feature setting.
- Add authorization, approval, denial, token, and cancellation operations.
- Add the **Add to Home Assistant** button to Settings and the wizard.
- Retain manual pairing under an advanced fallback.

### Home Assistant component

- Add Zeroconf metadata and `async_step_zeroconf`.
- Use the stable Kinosail server ID as the configuration-entry unique ID.
- Update the stored host when discovery finds the same server at a new address.
- Use the external-step approval flow and PKCE.
- Preserve manual URL and code reauthentication as fallbacks.

### Verification

- Test discovery, duplicates, changed addresses, denial, cancellation, expiry, replay, redirect mismatch, PKCE mismatch, and revocation.
- Test popup-blocked and manual flows.
- Test trusted and untrusted certificates without silently disabling verification.
- Test the Home Assistant browser and companion-app return paths.
- Prove that disabling the setting withdraws discovery and invalidates all grants.
