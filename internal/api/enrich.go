package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/razajahangir/devnull-tui/internal/config"
)

// inertiaVersion is the asset hash the website's Inertia front-end expects.
// It changes when they deploy; a 409 tells us the new value.
var inertiaVersion struct {
	mu  sync.Mutex
	val string
}

type inertiaPage struct {
	Props struct {
		Activity     *UserActivity     `json:"activity"`
		Streaks      *UserStreaks      `json:"streaks"`
		Achievements *UserAchievements `json:"achievements"`
	} `json:"props"`
}

// EnrichUser fills activity, streaks, and achievements from the public
// profile page. /api/v1/users/:id does not include those yet. Failure is
// ignored by the caller so a profile still opens without extras.
func (c *Client) EnrichUser(u *User) error {
	if u == nil || u.Username == "" {
		return fmt.Errorf("missing user")
	}
	raw := strings.TrimSpace(u.HTMLURL)
	if raw == "" {
		raw = c.BaseURL + "/@" + url.PathEscape(u.Username)
	}
	if err := sameOrigin(c.BaseURL, raw, c.AllowInsecureHTTP); err != nil {
		return err
	}
	body, err := c.getInertia(raw)
	if err != nil {
		return err
	}
	var page inertiaPage
	if err := json.Unmarshal(body, &page); err != nil {
		return fmt.Errorf("decode profile page: %w", err)
	}
	sanitizeWalk(&page)
	u.Activity = page.Props.Activity
	u.Streaks = page.Props.Streaks
	u.Achievements = page.Props.Achievements
	return nil
}

func (c *Client) getInertia(raw string) ([]byte, error) {
	ver := getInertiaVersion()
	body, status, newVer, err := c.inertiaOnce(raw, ver)
	if err != nil {
		return nil, err
	}
	if newVer != "" {
		setInertiaVersion(newVer)
	}
	if status == http.StatusConflict {
		ver = getInertiaVersion()
		body, status, _, err = c.inertiaOnce(raw, ver)
		if err != nil {
			return nil, err
		}
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("profile page HTTP %d", status)
	}
	return body, nil
}

func (c *Client) inertiaOnce(raw, version string) (body []byte, status int, newVer string, err error) {
	if err := config.CheckTransport(c.BaseURL, c.AllowInsecureHTTP); err != nil {
		return nil, 0, "", err
	}
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("X-Inertia", "true")
	if version != "" {
		req.Header.Set("X-Inertia-Version", version)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer res.Body.Close()
	newVer = res.Header.Get("X-Inertia-Version")
	limited := io.LimitReader(res.Body, MaxBodyBytes+1)
	rawBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, res.StatusCode, newVer, err
	}
	if len(rawBody) > MaxBodyBytes {
		return nil, res.StatusCode, newVer, fmt.Errorf("response larger than %d bytes", MaxBodyBytes)
	}
	return rawBody, res.StatusCode, newVer, nil
}

func getInertiaVersion() string {
	inertiaVersion.mu.Lock()
	defer inertiaVersion.mu.Unlock()
	return inertiaVersion.val
}

func setInertiaVersion(v string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return
	}
	inertiaVersion.mu.Lock()
	inertiaVersion.val = v
	inertiaVersion.mu.Unlock()
}

func sameOrigin(base, raw string, allowInsecure bool) error {
	b, err := url.Parse(base)
	if err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid profile url")
	}
	if u.User != nil {
		return fmt.Errorf("refusing profile url with credentials")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("refusing %s profile url", u.Scheme)
	}
	if !strings.EqualFold(u.Hostname(), b.Hostname()) {
		return fmt.Errorf("refusing off-instance profile url")
	}
	if u.Scheme == "http" && !(config.IsLoopback(u) || allowInsecure) {
		return fmt.Errorf("refusing insecure profile url")
	}
	return nil
}
