// Package auth implements the mobile-app PKCE sign-in flow.
//
// Start generates an Ed25519 device key and a PKCE verifier, then opens
// /mobile/authorize. The browser redirects to devnull://auth/callback.
// On Linux the TUI registers as that scheme's handler and writes the
// callback URL to a 0600 file; elsewhere the user pastes the URL.
//
// ParseCallback checks the OAuth state parameter before returning the
// authorization code. Redeeming the code (in package api) is what
// actually registers the device.
package auth
