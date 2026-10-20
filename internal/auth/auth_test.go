package auth

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseCallbackURL(t *testing.T) {
	code, err := ParseCallback("devnull://auth/callback?code=abc&state=xyz", "xyz")
	if err != nil {
		t.Fatal(err)
	}
	if code != "abc" {
		t.Fatalf("got %q", code)
	}
}

func TestParseCallbackBareCode(t *testing.T) {
	code, err := ParseCallback("abc", "xyz")
	if err != nil {
		t.Fatal(err)
	}
	if code != "abc" {
		t.Fatalf("got %q", code)
	}
}

func TestParseCallbackDenied(t *testing.T) {
	_, err := ParseCallback("devnull://auth/callback?error=access_denied&state=xyz", "xyz")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCallbackStateMismatch(t *testing.T) {
	_, err := ParseCallback("devnull://auth/callback?code=abc&state=nope", "xyz")
	if err == nil {
		t.Fatal("expected state mismatch")
	}
}

func TestPKCEChallengeLength(t *testing.T) {
	got := pkceChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if len(got) != 43 {
		t.Fatalf("challenge length %d: %s", len(got), got)
	}
	if got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("RFC 7636 appendix B mismatch: %s", got)
	}
}

func TestRandomStringUnbiasedBase64URL(t *testing.T) {
	s, err := randomString(48)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 64 {
		t.Fatalf("length %d", len(s))
	}
	if _, err := base64.RawURLEncoding.DecodeString(s); err != nil {
		t.Fatal(err)
	}
}

func TestAllowedBrowserURL(t *testing.T) {
	inst := "https://dev-null.sh"
	if err := AllowedBrowserURL(inst, "https://dev-null.sh/@me", false); err != nil {
		t.Fatal(err)
	}
	if err := AllowedBrowserURL(inst, "https://evil.example/x", false); err == nil {
		t.Fatal("third-party host")
	}
	if err := AllowedBrowserURL(inst, "file:///etc/passwd", false); err == nil {
		t.Fatal("file scheme")
	}
	if err := AllowedBrowserURL(inst, "http://dev-null.sh/x", false); err == nil {
		t.Fatal("http to production")
	}
	if err := AllowedBrowserURL("http://127.0.0.1:8000", "http://127.0.0.1:8000/x", false); err != nil {
		t.Fatal(err)
	}
}

func TestQuoteDesktopExec(t *testing.T) {
	got := quoteDesktopExec(`/home/a/bin$x%y` + "`z")
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Fatalf("not quoted: %s", got)
	}
	if !strings.Contains(got, `\$`) || !strings.Contains(got, `%%`) || !strings.Contains(got, "\\`") {
		t.Fatalf("missing escapes: %s", got)
	}
}
