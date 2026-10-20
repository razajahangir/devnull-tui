package auth

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/razajahangir/devnull-tui/internal/config"
)

func callbackPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "callback.url"), nil
}

// WriteCallback stores a protocol-handler callback for the running TUI to pick up.
func WriteCallback(raw string) error {
	path, err := callbackPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeAtomic(path, []byte(strings.TrimSpace(raw)+"\n"), 0o600)
}

// ReadCallback returns a pending callback URL and deletes the file.
func ReadCallback() (string, bool) {
	path, err := callbackPath()
	if err != nil {
		return "", false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	_ = os.Remove(path)
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", false
	}
	return value, true
}

// ClearCallback removes a leftover callback file from a previous attempt.
func ClearCallback() error {
	path, err := callbackPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RegisterProtocolHandler installs a Linux xdg-mime handler for devnull://.
func RegisterProtocolHandler() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	apps := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		return err
	}
	desktop := filepath.Join(apps, "devnull-tui.desktop")
	body := fmt.Sprintf("[Desktop Entry]\nVersion=1.0\nType=Application\nName=devnull TUI\nExec=%s callback %%u\nNoDisplay=true\nMimeType=x-scheme-handler/devnull;\n", quoteDesktopExec(exe))
	if err := writeAtomic(desktop, []byte(body), 0o644); err != nil {
		return err
	}
	_ = exec.Command("update-desktop-database", apps).Run()
	return exec.Command("xdg-mime", "default", "devnull-tui.desktop", "x-scheme-handler/devnull").Run()
}

// quoteDesktopExec quotes a path for a freedesktop Desktop Entry Exec line.
func quoteDesktopExec(path string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `\$`,
		"`", "\\`",
		`%`, `%%`,
	)
	return `"` + replacer.Replace(path) + `"`
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(name)
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
	if err := os.Rename(name, path); err != nil {
		return err
	}
	ok = true
	if runtime.GOOS != "windows" {
		return os.Chmod(path, perm)
	}
	return nil
}
