package api

import (
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server, ed25519.PrivateKey) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := New(srv.URL, priv)
	return c, srv, priv
}

func TestClientRefusesRemoteHTTP(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	c := New("http://example.invalid", priv)
	_, err := c.Me()
	if err == nil || !strings.Contains(err.Error(), "refusing http") {
		t.Fatalf("got %v", err)
	}
}

func TestClientAllowsLoopbackHTTP(t *testing.T) {
	c, _, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		if r.Header.Get("X-SSH-Signature") == "" {
			t.Error("missing signature")
		}
		_ = json.NewEncoder(w).Encode(MeResponse{Data: Me{User: User{Username: "me"}}})
	})
	me, err := c.Me()
	if err != nil {
		t.Fatal(err)
	}
	if me.Data.Username != "me" {
		t.Fatalf("%+v", me)
	}
}

func TestClientRefusesRedirect(t *testing.T) {
	c, _, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/other", http.StatusFound)
	})
	_, err := c.Me()
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("got %v", err)
	}
}

func TestClientCapsBody(t *testing.T) {
	c, _, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytesRepeat('a', MaxBodyBytes+2))
	})
	_, err := c.Me()
	if err == nil || !strings.Contains(err.Error(), "larger") {
		t.Fatalf("got %v", err)
	}
}

func TestPathEscapeUsername(t *testing.T) {
	var got string
	c, _, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RequestURI()
		_, _ = io.WriteString(w, `{"data":{"username":"a/b"}}`)
	})
	_, err := c.User("a/b")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/api/v1/users/a%2Fb" {
		t.Fatalf("path %q", got)
	}
}

func TestSanitizeOnDecode(t *testing.T) {
	c, _, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"id":"1","body":"hi\u001b[31mx","author":{"username":"a"}}}`)
	})
	page, err := c.Commit("1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page.Data.Body, "\x1b") {
		t.Fatalf("escape survived: %q", page.Data.Body)
	}
}

func TestErrorBodyTruncated(t *testing.T) {
	c, _, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = io.WriteString(w, "<html>"+strings.Repeat("e", 500)+"</html>")
	})
	_, err := c.Me()
	if err == nil {
		t.Fatal("expected error")
	}
	if len([]rune(err.Error())) > maxErrRunes+10 {
		t.Fatalf("error too long: %d", len([]rune(err.Error())))
	}
}

func bytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
