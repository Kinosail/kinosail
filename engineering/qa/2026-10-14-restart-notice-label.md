# Readable identity setting in the restart notice

Saving a synthetic OpenID Connect identity claim exposes `integrations.oidc.identity_claim` in the restart notice. The populated configuration baseline at `506fb01e` captures this on Settings. The notice should use the existing form label, `Single sign-on identity claim`.

Add the browser regression before changing product code. Save the synthetic configuration through the existing form, then inspect the restart notice on Settings and Advanced configuration. Cover 320px, 390px, 1440px, and 1920px widths. Retain eight screenshots and the displayed setting labels. Require the readable name and reject the raw internal key.

This assertion concerns the saved-but-not-active notice. It does not claim external identity-provider sign-in, a real Server restart, passkey behavior, or deployment acceptance. The owned fixture and existing 11-case selection remain unchanged. The fresh hosted baseline is pending. Local runtime remains on hold.
