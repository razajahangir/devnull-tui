package config

import (
	"crypto/ed25519"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const keyFileName = "device.ed25519"

// InstanceID is a filesystem-safe host[:port] used as the key directory name.
func InstanceID(rawURL string) (string, error) {
	u, err := ParseInstanceURL(rawURL)
	if err != nil {
		return "", err
	}
	id := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '<', '>', '"', '|', '?', '*':
			return '_'
		default:
			if r < 32 {
				return -1
			}
			return r
		}
	}, u.Host)
	id = strings.Trim(id, " .")
	if id == "" || id == "." || id == ".." {
		return "", fmt.Errorf("invalid instance host")
	}
	return id, nil
}

// KeyPath is ~/.config/devnull/instances/<host>/device.ed25519.
func KeyPath(rawURL string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	id, err := InstanceID(rawURL)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "instances", id, keyFileName), nil
}

// LoadKey returns the device key for this instance, migrating a legacy
// key file if one is still sitting in the config directory.
func LoadKey(rawURL string) (ed25519.PrivateKey, string, error) {
	path, err := KeyPath(rawURL)
	if err != nil {
		return nil, "", err
	}
	key, err := readKeyFile(path)
	if err == nil {
		return key, "", nil
	}
	if !os.IsNotExist(err) {
		return nil, "", err
	}

	dir, err := Dir()
	if err != nil {
		return nil, "", err
	}
	legacy := filepath.Join(dir, keyFileName)
	legacyKey, lerr := readKeyFile(legacy)
	if lerr != nil {
		if os.IsNotExist(lerr) {
			return nil, "", nil
		}
		return nil, "", lerr
	}
	if err := SaveKey(rawURL, legacyKey); err != nil {
		return nil, "", fmt.Errorf("migrate device key: %w", err)
	}
	_ = os.Remove(legacy)
	notice := "moved device key to " + path
	return legacyKey, notice, nil
}

// SaveKey writes the device private key for this instance atomically with mode 0600.
func SaveKey(rawURL string, priv ed25519.PrivateKey) error {
	if len(priv) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid ed25519 private key")
	}
	path, err := KeyPath(rawURL)
	if err != nil {
		return err
	}
	if err := secureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return writeFileAtomic(path, []byte(priv), 0o600)
}

// ForgetKey deletes the device key for this instance. Other instances are left alone.
func ForgetKey(rawURL string) error {
	path, err := KeyPath(rawURL)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func readKeyFile(path string) (ed25519.PrivateKey, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s is group- or world-readable; run chmod 600 %s", path, path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%s: expected %d-byte ed25519 private key", path, ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(raw), nil
}

func secureDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chmod(path, 0o700)
}

func writeFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if runtime.GOOS != "windows" {
		if err := tmp.Chmod(perm); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := replaceFile(tmpName, path); err != nil {
		return err
	}
	ok = true
	if runtime.GOOS != "windows" {
		return os.Chmod(path, perm)
	}
	return nil
}

// replaceFile moves tmpName onto path. Windows cannot rename over an existing file.
func replaceFile(tmpName, path string) error {
	err := os.Rename(tmpName, path)
	if err == nil || runtime.GOOS != "windows" {
		return err
	}
	if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
		return err
	}
	return os.Rename(tmpName, path)
}
