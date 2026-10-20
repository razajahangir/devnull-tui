package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultsAreProduction(t *testing.T) {
	cfg := Defaults()
	if cfg.URL != DefaultURL {
		t.Fatalf("default url %q", cfg.URL)
	}
	if !cfg.IsProduction() {
		t.Fatal("defaults should be production")
	}
	if cfg.PollInterval != DefaultPollInterval {
		t.Fatalf("poll %s", cfg.PollInterval)
	}
	if cfg.AutoLoadNew {
		t.Fatal("auto_load_new should default off")
	}
	if !cfg.RefreshOnSwitch {
		t.Fatal("refresh_on_switch should default on")
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	clearEnv(t)
	res, err := Load(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Config.URL != DefaultURL || res.Sources.URL != srcDefault {
		t.Fatalf("%+v", res)
	}
}

func TestPrecedenceFlagBeatsEnvBeatsFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	clearEnv(t)
	path := filepath.Join(dir, "devnull", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{
  "url": "https://file.example",
  "poll_interval": "45s",
  "auto_load_new": true,
  "refresh_on_switch": false,
  "mouse": false
}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DEVNULL_URL", "https://env.example")
	t.Setenv("DEVNULL_POLL_INTERVAL", "1m")

	res, err := Load(Overrides{URL: "https://flag.example", PollInterval: "2m"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Config.URL != "https://flag.example" || res.Sources.URL != srcFlag {
		t.Fatalf("url %+v", res)
	}
	if res.Config.PollInterval != 2*time.Minute || res.Sources.PollInterval != srcFlag {
		t.Fatalf("poll %+v", res)
	}
	if !res.Config.AutoLoadNew || res.Sources.AutoLoadNew != srcFile {
		t.Fatalf("auto_load %+v", res)
	}
}

func TestUnknownKeyRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.json")
	if err := os.WriteFile(path, []byte(`{"pol_interval":"30s"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	clearEnv(t)
	_, err := Load(Overrides{ConfigPath: path})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("got %v", err)
	}
}

func TestPollIntervalBounds(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	_, err := Load(Overrides{PollInterval: "1s"})
	if err == nil {
		t.Fatal("expected too-short interval to fail")
	}
	_, err = Load(Overrides{PollInterval: "11m"})
	if err == nil {
		t.Fatal("expected too-long interval to fail")
	}
}

func TestURLRejectsPathAndUser(t *testing.T) {
	cases := []string{
		"https://dev-null.sh/api",
		"https://user:pass@dev-null.sh",
		"https://dev-null.sh?x=1",
		"ftp://dev-null.sh",
		"not-a-url",
	}
	for _, c := range cases {
		if _, err := ParseInstanceURL(c); err == nil {
			t.Errorf("accepted %q", c)
		}
	}
}

func TestHTTPRejectedUnlessLoopbackOrFlag(t *testing.T) {
	if err := CheckTransport("http://example.com", false); err == nil {
		t.Fatal("remote http should fail")
	}
	if err := CheckTransport("http://example.com", true); err != nil {
		t.Fatal(err)
	}
	if err := CheckTransport("http://127.0.0.1:8000", false); err != nil {
		t.Fatal(err)
	}
	if err := CheckTransport("http://localhost:8000", false); err != nil {
		t.Fatal(err)
	}
	if err := CheckTransport("https://dev-null.sh", false); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyBaseURL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	clearEnv(t)
	path := filepath.Join(dir, "devnull", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"base_url":"https://legacy.example","device_name":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Load(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Config.URL != "https://legacy.example" {
		t.Fatalf("url %q", res.Config.URL)
	}
	if res.Sources.URL != srcLegacy {
		t.Fatalf("source %q", res.Sources.URL)
	}
	if res.Notice == "" {
		t.Fatal("expected migration notice")
	}
	if res.Config.DeviceName != "old" {
		t.Fatalf("device %q", res.Config.DeviceName)
	}
}

func TestLoadDoesNotWriteConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	clearEnv(t)
	if _, err := Load(Overrides{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "devnull", "config.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Load must not create config.json")
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"DEVNULL_CONFIG", "DEVNULL_URL", "DEVNULL_POLL_INTERVAL",
		"DEVNULL_AUTO_LOAD_NEW", "DEVNULL_REFRESH_ON_SWITCH",
		"DEVNULL_MOUSE", "DEVNULL_SHOW_IMAGES", "DEVNULL_ALLOW_INSECURE_HTTP", "DEVNULL_DEVICE_NAME",
	} {
		t.Setenv(k, "")
	}
}
