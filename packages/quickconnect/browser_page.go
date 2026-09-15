package quickconnect

// BrowserPage supplies app branding and existing federated sign-in choices.
type BrowserPage struct {
	Product, Next string
	OIDC, SAML    bool
}

// BrowserHTML keeps public sign-in separate from local password and account management.
const BrowserHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Sign in away from home · {{.Product}}</title><link rel="icon" href="/static/icon.svg"><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1">
<script defer src="/static/passkeys.js?v=13"></script><script defer src="/static/public-login.js?v=1"></script></head>
<body class="auth" data-login-next="{{.Next}}"><a class="skip" href="#main">Skip to content</a><main id="main" class="grant-card">
<img class="brand-mark" src="/static/icon.svg" alt=""><h1>Sign in away from home</h1><p>{{.Product}} gives you access to your Viewer's allowed Libraries.</p>
<section aria-labelledby="connect-heading"><h2 id="connect-heading">Connect this browser</h2><p>Ask someone signed in as your Viewer at home to approve a code. They must use an authenticator or passkey to confirm it's them.</p>
<button type="button" data-public-start disabled>Get a sign-in code</button>
<div data-public-pending hidden><p tabindex="-1" data-public-code-label>Your sign-in code: <strong><output data-public-code></output></strong></p><ol><li>On the home device, sign in as your remote-enabled Viewer.</li><li>Open Quick Connect and enter this code. Approve only if the code and requesting device match.</li><li>Keep this page open. Your library opens after approval.</li></ol><p>Codes expire after a few minutes. Never share a code you did not request.</p><button class="quiet" type="button" data-public-cancel>Cancel sign-in</button></div>
<p role="status" aria-live="polite" data-public-status></p><noscript><p>Enable JavaScript to use Quick Connect, or connect through the Kinosail app.</p></noscript></section>
<details><summary>Already have another sign-in method?</summary><p>Use a passkey already registered for this public Server address.</p><button class="quiet" type="button" data-passkey-login>Sign in with passkey</button><output role="status" data-passkey-status></output>{{if .OIDC}}<p><a href="/api/v1/session/oidc">Sign in with OpenID Connect</a></p>{{end}}{{if .SAML}}<p><a href="/api/v1/session/saml">Sign in with SAML</a></p>{{end}}</details>
<p>Owner settings, passwords, and adding sign-in methods are available on your home network or WireGuard.</p>{{languagePicker}}</main></body></html>`
