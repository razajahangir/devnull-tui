package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultURL is the production /dev/null instance. Forks that talk to a
// different server by default should change only this constant.
const DefaultURL = "https://dev-null.sh"

const (
	// DefaultPollInterval is how often the TUI checks for new commits.
	DefaultPollInterval = 30 * time.Second
	// MinPollInterval is the shortest interval accepted in config.
	MinPollInterval = 10 * time.Second
	// MaxPollInterval is the longest interval accepted in config.
	MaxPollInterval = 10 * time.Minute
)

const (
	srcDefault = "default"
	srcFile    = "file"
	srcEnv     = "env"
	srcFlag    = "flag"
	srcLegacy  = "legacy config"
)

// Config is the resolved runtime settings. Zero values are not valid;
// use Defaults or Load.
type Config struct {
	// URL is the instance origin, with no path, query, or credentials.
	URL string
	// PollInterval is how often background polls run.
	PollInterval time.Duration
	// AutoLoadNew loads new commits when the cursor is at the top of a feed.
	AutoLoadNew bool
	// RefreshOnSwitch reloads a tab when you switch to it if it is stale.
	RefreshOnSwitch bool
	// Mouse enables wheel scrolling.
	Mouse bool
	// AllowInsecureHTTP permits http:// to non-loopback hosts. Off by default.
	AllowInsecureHTTP bool
	// DeviceName is shown on the instance's device list at sign-in.
	DeviceName string
	// Path is the settings file that was read, or the default path if none exists.
	Path string
}

// Sources records where each setting came from (flag, env, file, default).
type Sources struct {
	URL               string
	PollInterval      string
	AutoLoadNew       string
	RefreshOnSwitch   string
	Mouse             string
	AllowInsecureHTTP string
	DeviceName        string
}

// Result is a fully resolved Config plus provenance for `devnull config`.
type Result struct {
	Config  Config
	Sources Sources
	// Notice is a one-line warning for stderr, or empty.
	Notice string
}

// Overrides are values supplied from flags. Empty strings mean "not set".
type Overrides struct {
	ConfigPath        string
	URL               string
	PollInterval      string
	AutoLoadNew       string
	RefreshOnSwitch   string
	Mouse             string
	AllowInsecureHTTP string
	DeviceName        string
}

type fileConfig struct {
	URL               *string `json:"url"`
	BaseURL           *string `json:"base_url"` // legacy, read-only
	PollInterval      *string `json:"poll_interval"`
	AutoLoadNew       *bool   `json:"auto_load_new"`
	RefreshOnSwitch   *bool   `json:"refresh_on_switch"`
	Mouse             *bool   `json:"mouse"`
	ShowImages        *bool   `json:"show_images"` // ignored; kept so older config files still load
	AllowInsecureHTTP *bool   `json:"allow_insecure_http"`
	DeviceName        *string `json:"device_name"`
}

// Defaults returns production settings.
func Defaults() Config {
	return Config{
		URL:               DefaultURL,
		PollInterval:      DefaultPollInterval,
		AutoLoadNew:       false,
		RefreshOnSwitch:   true,
		Mouse:             true,
		AllowInsecureHTTP: false,
		DeviceName:        hostnameDeviceName(),
	}
}

// Dir is the per-user config directory: $XDG_CONFIG_HOME/devnull or ~/.config/devnull.
func Dir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "devnull"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "devnull"), nil
}

// DefaultPath is the settings file path.
func DefaultPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load resolves settings from file, environment, and flag overrides.
func Load(o Overrides) (Result, error) {
	cfg := Defaults()
	src := Sources{
		URL:               srcDefault,
		PollInterval:      srcDefault,
		AutoLoadNew:       srcDefault,
		RefreshOnSwitch:   srcDefault,
		Mouse:             srcDefault,
		AllowInsecureHTTP: srcDefault,
		DeviceName:        srcDefault,
	}

	path := o.ConfigPath
	if path == "" {
		path = strings.TrimSpace(os.Getenv("DEVNULL_CONFIG"))
	}
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Result{}, err
		}
	}
	cfg.Path = path

	var notice string
	if raw, err := os.ReadFile(path); err == nil {
		fc, err := decodeFile(raw)
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", path, err)
		}
		if fc.URL != nil && strings.TrimSpace(*fc.URL) != "" {
			cfg.URL = strings.TrimSpace(*fc.URL)
			src.URL = srcFile
		} else if fc.BaseURL != nil && strings.TrimSpace(*fc.BaseURL) != "" {
			cfg.URL = strings.TrimSpace(*fc.BaseURL)
			src.URL = srcLegacy
			notice = "using legacy base_url from " + path + "; add a \"url\" field (or delete the file) to use the production default"
		}
		if fc.PollInterval != nil {
			d, err := parsePollInterval(*fc.PollInterval)
			if err != nil {
				return Result{}, fmt.Errorf("%s: poll_interval: %w", path, err)
			}
			cfg.PollInterval = d
			src.PollInterval = srcFile
		}
		if fc.AutoLoadNew != nil {
			cfg.AutoLoadNew = *fc.AutoLoadNew
			src.AutoLoadNew = srcFile
		}
		if fc.RefreshOnSwitch != nil {
			cfg.RefreshOnSwitch = *fc.RefreshOnSwitch
			src.RefreshOnSwitch = srcFile
		}
		if fc.Mouse != nil {
			cfg.Mouse = *fc.Mouse
			src.Mouse = srcFile
		}
		if fc.AllowInsecureHTTP != nil {
			cfg.AllowInsecureHTTP = *fc.AllowInsecureHTTP
			src.AllowInsecureHTTP = srcFile
		}
		if fc.DeviceName != nil && strings.TrimSpace(*fc.DeviceName) != "" {
			cfg.DeviceName = strings.TrimSpace(*fc.DeviceName)
			src.DeviceName = srcFile
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}

	if err := applyEnv(&cfg, &src); err != nil {
		return Result{}, err
	}

	if err := applyOverride(&cfg.URL, &src.URL, o.URL, parseURLString); err != nil {
		return Result{}, fmt.Errorf("--url: %w", err)
	}
	if o.PollInterval != "" {
		d, err := parsePollInterval(o.PollInterval)
		if err != nil {
			return Result{}, fmt.Errorf("--poll-interval: %w", err)
		}
		cfg.PollInterval = d
		src.PollInterval = srcFlag
	}
	if err := applyBoolOverride(&cfg.AutoLoadNew, &src.AutoLoadNew, o.AutoLoadNew); err != nil {
		return Result{}, fmt.Errorf("--auto-load-new: %w", err)
	}
	if err := applyBoolOverride(&cfg.RefreshOnSwitch, &src.RefreshOnSwitch, o.RefreshOnSwitch); err != nil {
		return Result{}, fmt.Errorf("--refresh-on-switch: %w", err)
	}
	if err := applyBoolOverride(&cfg.Mouse, &src.Mouse, o.Mouse); err != nil {
		return Result{}, fmt.Errorf("--mouse: %w", err)
	}
	if err := applyBoolOverride(&cfg.AllowInsecureHTTP, &src.AllowInsecureHTTP, o.AllowInsecureHTTP); err != nil {
		return Result{}, fmt.Errorf("--allow-insecure-http: %w", err)
	}
	if o.DeviceName != "" {
		cfg.DeviceName = o.DeviceName
		src.DeviceName = srcFlag
	}

	u, err := ParseInstanceURL(cfg.URL)
	if err != nil {
		return Result{}, fmt.Errorf("url: %w", err)
	}
	cfg.URL = strings.TrimRight(u.String(), "/")
	if err := CheckTransport(cfg.URL, cfg.AllowInsecureHTTP); err != nil {
		return Result{}, err
	}

	return Result{Config: cfg, Sources: src, Notice: notice}, nil
}

func decodeFile(raw []byte) (fileConfig, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	var fc fileConfig
	if err := dec.Decode(&fc); err != nil {
		return fileConfig{}, err
	}
	// Reject trailing junk.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fileConfig{}, fmt.Errorf("trailing data after JSON object")
		}
		return fileConfig{}, err
	}
	return fc, nil
}

func applyEnv(cfg *Config, src *Sources) error {
	if v := strings.TrimSpace(os.Getenv("DEVNULL_URL")); v != "" {
		if err := applyOverride(&cfg.URL, &src.URL, v, parseURLString); err != nil {
			return fmt.Errorf("DEVNULL_URL: %w", err)
		}
		src.URL = srcEnv
	}
	if v := strings.TrimSpace(os.Getenv("DEVNULL_POLL_INTERVAL")); v != "" {
		d, err := parsePollInterval(v)
		if err != nil {
			return fmt.Errorf("DEVNULL_POLL_INTERVAL: %w", err)
		}
		cfg.PollInterval = d
		src.PollInterval = srcEnv
	}
	if err := applyBoolEnv(&cfg.AutoLoadNew, &src.AutoLoadNew, "DEVNULL_AUTO_LOAD_NEW"); err != nil {
		return err
	}
	if err := applyBoolEnv(&cfg.RefreshOnSwitch, &src.RefreshOnSwitch, "DEVNULL_REFRESH_ON_SWITCH"); err != nil {
		return err
	}
	if err := applyBoolEnv(&cfg.Mouse, &src.Mouse, "DEVNULL_MOUSE"); err != nil {
		return err
	}
	if err := applyBoolEnv(&cfg.AllowInsecureHTTP, &src.AllowInsecureHTTP, "DEVNULL_ALLOW_INSECURE_HTTP"); err != nil {
		return err
	}
	if v := strings.TrimSpace(os.Getenv("DEVNULL_DEVICE_NAME")); v != "" {
		cfg.DeviceName = v
		src.DeviceName = srcEnv
	}
	return nil
}

func applyOverride(dst *string, src *string, val string, parse func(string) (string, error)) error {
	if val == "" {
		return nil
	}
	parsed, err := parse(val)
	if err != nil {
		return err
	}
	*dst = parsed
	*src = srcFlag
	return nil
}

func applyBoolOverride(dst *bool, src *string, val string) error {
	if val == "" {
		return nil
	}
	b, err := parseBool(val)
	if err != nil {
		return err
	}
	*dst = b
	*src = srcFlag
	return nil
}

func applyBoolEnv(dst *bool, src *string, key string) error {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return nil
	}
	b, err := parseBool(v)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	*dst = b
	*src = srcEnv
	return nil
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	}
	return false, fmt.Errorf("not a boolean: %q", s)
}

func parseURLString(s string) (string, error) {
	u, err := ParseInstanceURL(s)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func parsePollInterval(s string) (time.Duration, error) {
	d, err := time.ParseDuration(strings.TrimSpace(s))
	if err != nil {
		// Allow a bare number of seconds.
		if n, nerr := strconv.Atoi(strings.TrimSpace(s)); nerr == nil {
			d = time.Duration(n) * time.Second
		} else {
			return 0, fmt.Errorf("invalid duration %q (want 30s, 1m, …)", s)
		}
	}
	if d < MinPollInterval || d > MaxPollInterval {
		return 0, fmt.Errorf("must be between %s and %s", MinPollInterval, MaxPollInterval)
	}
	return d, nil
}

// ParseInstanceURL validates an instance origin: http(s), host, no path/user/query.
func ParseInstanceURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("must be http or https")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("must include a host")
	}
	if u.User != nil {
		return nil, fmt.Errorf("must not include credentials")
	}
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("must not include a path")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("must not include a query or fragment")
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}

// CheckTransport rejects remote http:// unless AllowInsecureHTTP is set.
func CheckTransport(raw string, allowInsecure bool) error {
	u, err := ParseInstanceURL(raw)
	if err != nil {
		return err
	}
	if u.Scheme == "https" {
		return nil
	}
	if IsLoopback(u) || allowInsecure {
		return nil
	}
	return fmt.Errorf("refusing http://%s (pass --allow-insecure-http or use https)", u.Host)
}

// IsLoopback reports whether u points at localhost.
func IsLoopback(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// IsProduction reports whether this config talks to the official instance.
func (c Config) IsProduction() bool {
	u, err := url.Parse(c.URL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "dev-null.sh" || host == "www.dev-null.sh"
}

// Format explains resolved settings for `devnull config`.
func (r Result) Format() string {
	c := r.Config
	s := r.Sources
	var b strings.Builder
	row := func(name, value, source string) {
		fmt.Fprintf(&b, "%-22s %-36s (%s)\n", name, value, source)
	}
	row("url", c.URL, s.URL)
	row("poll_interval", c.PollInterval.String(), s.PollInterval)
	row("auto_load_new", strconv.FormatBool(c.AutoLoadNew), s.AutoLoadNew)
	row("refresh_on_switch", strconv.FormatBool(c.RefreshOnSwitch), s.RefreshOnSwitch)
	row("mouse", strconv.FormatBool(c.Mouse), s.Mouse)
	row("allow_insecure_http", strconv.FormatBool(c.AllowInsecureHTTP), s.AllowInsecureHTTP)
	row("device_name", c.DeviceName, s.DeviceName)
	fmt.Fprintf(&b, "\nconfig file            %s\n", c.Path)
	if keyPath, err := KeyPath(c.URL); err == nil {
		state := "absent"
		if _, err := os.Stat(keyPath); err == nil {
			state = "present"
		}
		fmt.Fprintf(&b, "device key             %s (%s)\n", keyPath, state)
	}
	if r.Notice != "" {
		fmt.Fprintf(&b, "\nnote: %s\n", r.Notice)
	}
	return b.String()
}

func hostnameDeviceName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "devnull TUI"
	}
	return "devnull TUI on " + host
}
