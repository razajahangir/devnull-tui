package config

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveAndLoadKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveKey(DefaultURL, priv); err != nil {
		t.Fatal(err)
	}
	got, notice, err := LoadKey(DefaultURL)
	if err != nil || notice != "" {
		t.Fatalf("err=%v notice=%q", err, notice)
	}
	if string(got) != string(priv) {
		t.Fatal("key mismatch")
	}
	path, err := KeyPath(DefaultURL)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("perm %o", fi.Mode().Perm())
	}
}

func TestKeysArePerInstance(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	_, a, _ := ed25519.GenerateKey(nil)
	_, b, _ := ed25519.GenerateKey(nil)
	if err := SaveKey("https://dev-null.sh", a); err != nil {
		t.Fatal(err)
	}
	if err := SaveKey("http://127.0.0.1:8000", b); err != nil {
		t.Fatal(err)
	}
	gotA, _, err := LoadKey("https://dev-null.sh")
	if err != nil {
		t.Fatal(err)
	}
	gotB, _, err := LoadKey("http://127.0.0.1:8000")
	if err != nil {
		t.Fatal(err)
	}
	if string(gotA) != string(a) || string(gotB) != string(b) {
		t.Fatal("keys mixed across instances")
	}
	if err := ForgetKey("https://dev-null.sh"); err != nil {
		t.Fatal(err)
	}
	gone, _, err := LoadKey("https://dev-null.sh")
	if err != nil || gone != nil {
		t.Fatalf("production key should be gone: %v %v", gone, err)
	}
	still, _, err := LoadKey("http://127.0.0.1:8000")
	if err != nil || string(still) != string(b) {
		t.Fatal("localhost key should remain")
	}
}

func TestLegacyKeyMigration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	_, priv, _ := ed25519.GenerateKey(nil)
	legacyDir := filepath.Join(dir, "devnull")
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(legacyDir, "device.ed25519")
	if err := os.WriteFile(legacy, priv, 0o600); err != nil {
		t.Fatal(err)
	}
	got, notice, err := LoadKey(DefaultURL)
	if err != nil {
		t.Fatal(err)
	}
	if notice == "" {
		t.Fatal("expected migration notice")
	}
	if string(got) != string(priv) {
		t.Fatal("migrated key mismatch")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy key should be removed")
	}
	path, _ := KeyPath(DefaultURL)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestRejectWorldReadableKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix permissions")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	_, priv, _ := ed25519.GenerateKey(nil)
	if err := SaveKey(DefaultURL, priv); err != nil {
		t.Fatal(err)
	}
	path, _ := KeyPath(DefaultURL)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadKey(DefaultURL)
	if err == nil {
		t.Fatal("expected error for a world-readable key")
	}
}
