package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/razajahangir/devnull-tui/internal/config"
	"github.com/razajahangir/devnull-tui/internal/sign"
)

// RedirectURI is the custom-scheme callback the mobile app also uses.
const RedirectURI = "devnull://auth/callback"

// Session is an in-progress sign-in. The private key is written to disk
// only after the server accepts the authorization code.
type Session struct {
	Private      ed25519.PrivateKey
	Verifier     string
	State        string
	AuthorizeURL string
}

// Start creates a device key and the browser approval URL.
func Start(baseURL, deviceName string) (Session, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Session{}, err
	}
	verifier, err := randomString(48)
	if err != nil {
		return Session{}, err
	}
	state, err := randomString(24)
	if err != nil {
		return Session{}, err
	}
	_ = ClearCallback()
	pub := priv.Public().(ed25519.PublicKey)
	challenge := pkceChallenge(verifier)
	q := url.Values{}
	q.Set("fingerprint", sign.Fingerprint(pub))
	q.Set("device_name", deviceName)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("redirect_uri", RedirectURI)
	authorize := strings.TrimRight(baseURL, "/") + "/mobile/authorize?" + q.Encode()
	return Session{
		Private:      priv,
		Verifier:     verifier,
		State:        state,
		AuthorizeURL: authorize,
	}, nil
}

// ParseCallback extracts the authorization code from a pasted or
// protocol-handler URL. A bare code is accepted for copy-paste fallback.
func ParseCallback(raw, expectedState string) (code string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("paste the devnull:// URL from the browser")
	}
	if !strings.Contains(raw, "://") {
		return raw, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("could not parse callback: %w", err)
	}
	if u.Query().Get("error") != "" {
		return "", fmt.Errorf("authorization %s", u.Query().Get("error"))
	}
	state := u.Query().Get("state")
	if expectedState != "" && state != expectedState {
		return "", fmt.Errorf("state mismatch; start login again")
	}
	code = u.Query().Get("code")
	if code == "" {
		return "", fmt.Errorf("callback is missing code")
	}
	return code, nil
}

// AllowedBrowserURL reports whether target may be passed to the OS opener.
// Only http(s) URLs on the configured instance host are allowed, so a
// crafted html_url cannot launch file:// or a third-party site.
func AllowedBrowserURL(instanceURL, target string, allowInsecureHTTP bool) error {
	base, err := config.ParseInstanceURL(instanceURL)
	if err != nil {
		return fmt.Errorf("instance url: %w", err)
	}
	u, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid link")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("refusing to open %s link", u.Scheme)
	}
	if u.Scheme == "http" && !config.IsLoopback(u) && !allowInsecureHTTP {
		return fmt.Errorf("refusing to open insecure http link")
	}
	if !strings.EqualFold(u.Hostname(), base.Hostname()) {
		return fmt.Errorf("refusing to open a link on %s (this instance is %s)", u.Hostname(), base.Hostname())
	}
	return nil
}

// OpenBrowser asks the OS to open rawURL. Call AllowedBrowserURL first
// for server-supplied URLs.
func OpenBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// randomString returns n random bytes as unpadded base64url (RFC 7636).
func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
