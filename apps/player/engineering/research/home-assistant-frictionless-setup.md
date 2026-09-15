# Frictionless Home Assistant setup

Research date: 2026-08-27

## Decision

Use this primary connection flow:

1. Kinosail advertises `_kinosail._tcp.local.` only while Home Assistant access is enabled.
2. Home Assistant discovers the Server and asks for confirmation.
3. Home Assistant opens a Kinosail approval page in a new window.
4. A signed-in Kinosail Owner approves the connection.
5. Kinosail returns a short-lived authorization code.
6. Home Assistant exchanges it with Proof Key for Code Exchange (PKCE).
7. Home Assistant stores the narrow grant and closes the window.

Keep the eight-digit pairing code as a manual fallback.

Home Assistant calls this popup pattern an external step. Its frontend opens the URL with `window.open`. The callback advances the flow and returns `window.close()` ([external-step documentation](https://developers.home-assistant.io/docs/data_entry_flow_index/#external-step--external-step-done), [frontend implementation](https://github.com/home-assistant/frontend/blob/dev/src/dialogs/config-flow/step-flow-external.ts)).

## Design reasons

The normal flow needs two approvals after installation:

- Home Assistant: connect the discovered Kinosail Server.
- Kinosail: allow Home Assistant.

The user does not copy a password, token, address, or code. Home Assistant never receives the Owner password.

Home Assistant supports Zeroconf discovery through the integration manifest and `async_step_zeroconf`. Discovery must use a stable Server ID and still require confirmation ([config-flow discovery](https://developers.home-assistant.io/docs/core/integration/config_flow/#discovery-steps), [Zeroconf manifest](https://developers.home-assistant.io/docs/creating_integration_manifest/#zeroconf)). Multicast Domain Name System data locates Kinosail. It does not prove identity.

## Authorization

Use Authorization Code with PKCE S256. Home Assistant provides `LocalOAuth2ImplementationWithPkce`, including its signed callback state, verifier, and challenge ([Home Assistant OAuth helper](https://github.com/home-assistant/core/blob/dev/homeassistant/helpers/config_entry_oauth2_flow.py), [PKCE guidance](https://developers.home-assistant.io/docs/core/platform/application_credentials/)).

Kinosail must:

- require an authenticated Owner for approval;
- show the exact narrow permissions;
- provide explicit allow and cancel actions;
- bind the code to one transaction, redirect URI, and PKCE challenge;
- expire the code quickly and permit one exchange;
- keep credentials out of URLs; and
- revoke all grants when the setting is disabled.

These rules follow current OAuth guidance ([RFC 7636](https://www.rfc-editor.org/rfc/rfc7636), [RFC 9700](https://www.rfc-editor.org/rfc/rfc9700)).

Kinosail uses a fixed public client. It does not require each household to create a client ID or secret.

## Kinosail entry point

The enabled Settings and wizard states link to:

`https://my.home-assistant.io/redirect/config_flow_start/?domain=kinosail`

My Home Assistant selects the user's saved instance without sending that instance URL to Kinosail ([My Home Assistant](https://www.home-assistant.io/integrations/my/)). The redirect accepts only the integration domain. It cannot carry a Server address or credential, so local discovery remains important ([redirect catalog](https://github.com/home-assistant/my.home-assistant.io/blob/main/redirect.json)).

The link starts the flow only after the custom integration is installed.

## Installation

The complete distribution path is:

1. Publish a versioned GitHub release for the component.
2. Run Home Assistant Community Store (HACS) and Hassfest validation.
3. Submit the repository for the default HACS list.
4. Consider Home Assistant Core inclusion after the integration has sufficient usage and maturity.

Until HACS accepts the repository, users add it as a custom repository ([custom repositories](https://www.hacs.xyz/docs/faq/custom_repositories/), [default listing requirements](https://www.hacs.xyz/docs/publish/include/)).

## Fallbacks

Use these fallbacks in order:

1. Manual Server address when mDNS cannot cross the container network.
2. Browser approval after manual Server selection.
3. The eight-digit pairing code when browser approval is unavailable.

Do not add a device authorization flow until real use shows that these paths are insufficient.
