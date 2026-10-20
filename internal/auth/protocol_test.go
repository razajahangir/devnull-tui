package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCallbackFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	if err := WriteCallback("devnull://auth/callback?code=abc&state=xyz"); err != nil {
		t.Fatal(err)
	}
	got, ok := ReadCallback()
	if !ok || got != "devnull://auth/callback?code=abc&state=xyz" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	if _, err := os.Stat(filepath.Join(dir, "devnull", "callback.url")); !os.IsNotExist(err) {
		t.Fatal("callback file should be removed after read")
	}
}
