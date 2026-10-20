package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/razajahangir/devnull-tui/internal/config"
	"github.com/razajahangir/devnull-tui/internal/sign"
)

const (
	// MaxBodyBytes is the largest response we will read. Anything bigger
	// is treated as an error so a misbehaving server cannot fill memory.
	MaxBodyBytes = 4 << 20
	maxErrRunes  = 200
)

// UserAgent is sent on every request. main sets this from the build version.
var UserAgent = "devnull-tui/dev"

// Client is a signed /api/v1 client for one instance.
type Client struct {
	BaseURL           string
	Key               ed25519.PrivateKey
	HTTP              *http.Client
	AllowInsecureHTTP bool
}

// New builds a client. baseURL must already have been validated by config.
func New(baseURL string, key ed25519.PrivateKey) *Client {
	return NewWithOptions(baseURL, key, false)
}

// NewWithOptions is New plus the insecure-HTTP toggle from settings.
func NewWithOptions(baseURL string, key ed25519.PrivateKey, allowInsecure bool) *Client {
	return &Client{
		BaseURL:           strings.TrimRight(baseURL, "/"),
		Key:               key,
		AllowInsecureHTTP: allowInsecure,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return fmt.Errorf("refusing HTTP redirect")
			},
		},
	}
}

// Me returns the signed-in user.
func (c *Client) Me() (MeResponse, error) {
	var out MeResponse
	err := c.do("GET", "/api/v1/me", nil, &out)
	return out, err
}

// RedeemDevice finishes PKCE sign-in and returns the new session's user.
func (c *Client) RedeemDevice(code, verifier string) (MeResponse, error) {
	var out MeResponse
	err := c.do("POST", "/api/v1/devices", map[string]string{
		"code":          code,
		"code_verifier": verifier,
	}, &out)
	return out, err
}

// Log fetches a feed page. kind is following, latest, or trending.
func (c *Client) Log(kind, cursor, since string) (LogPage, error) {
	q := url.Values{}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if since != "" {
		q.Set("since", since)
	}
	path := apiPath("log", kind)
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var out LogPage
	err := c.do("GET", path, nil, &out)
	return out, err
}

// Commit fetches one commit and its ancestors.
func (c *Client) Commit(id string) (CommitPage, error) {
	var out CommitPage
	err := c.do("GET", apiPath("commits", id), nil, &out)
	return out, err
}

// Replies fetches a page of replies.
func (c *Client) Replies(id, cursor string, newest bool) (Page[Commit], error) {
	q := url.Values{}
	if newest {
		q.Set("newest", "1")
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	path := apiPath("commits", id, "replies")
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var out Page[Commit]
	err := c.do("GET", path, nil, &out)
	return out, err
}

// CreateCommit posts a new commit or reply.
func (c *Client) CreateCommit(body string, parentID, quotedID *string) (Commit, error) {
	var out struct {
		Data Commit `json:"data"`
	}
	err := c.do("POST", "/api/v1/commits", CreateCommit{
		Body:           body,
		ParentID:       parentID,
		QuotedCommitID: quotedID,
	}, &out)
	return out.Data, err
}

// DeleteCommit deletes one of the signed-in user's commits.
func (c *Client) DeleteCommit(id string) error {
	return c.do("DELETE", apiPath("commits", id), nil, nil)
}

// Star adds or removes a star.
func (c *Client) Star(id string, on bool) (Commit, error) {
	method := "POST"
	if !on {
		method = "DELETE"
	}
	var out struct {
		Data Commit `json:"data"`
	}
	err := c.do(method, apiPath("commits", id, "star"), nil, &out)
	return out.Data, err
}

// Bookmark adds or removes a save.
func (c *Client) Bookmark(id string, on bool) (Commit, error) {
	method := "PUT"
	if !on {
		method = "DELETE"
	}
	var out struct {
		Data Commit `json:"data"`
	}
	err := c.do(method, apiPath("commits", id, "bookmark"), nil, &out)
	return out.Data, err
}

// Follow follows or unfollows a user.
func (c *Client) Follow(username string, on bool) (User, error) {
	method := "POST"
	if !on {
		method = "DELETE"
	}
	var out struct {
		Data User `json:"data"`
	}
	err := c.do(method, apiPath("users", username, "follow"), nil, &out)
	return out.Data, err
}

// Mute mutes or unmutes a user.
func (c *Client) Mute(username string, on bool) (User, error) {
	method := "POST"
	if !on {
		method = "DELETE"
	}
	var out struct {
		Data User `json:"data"`
	}
	err := c.do(method, apiPath("users", username, "mute"), nil, &out)
	return out.Data, err
}

// Block blocks or unblocks a user.
func (c *Client) Block(username string, on bool) (User, error) {
	method := "POST"
	if !on {
		method = "DELETE"
	}
	var out struct {
		Data User `json:"data"`
	}
	err := c.do(method, apiPath("users", username, "block"), nil, &out)
	return out.Data, err
}

// User fetches a public profile.
func (c *Client) User(username string) (User, error) {
	var out struct {
		Data User `json:"data"`
	}
	err := c.do("GET", apiPath("users", username), nil, &out)
	return out.Data, err
}

// UserCommits fetches a user's commit log.
func (c *Client) UserCommits(username, cursor string) (Page[Commit], error) {
	path := apiPath("users", username, "commits")
	if cursor != "" {
		path += "?cursor=" + url.QueryEscape(cursor)
	}
	var out Page[Commit]
	err := c.do("GET", path, nil, &out)
	return out, err
}

// Notifications fetches the signed-in user's notifications.
func (c *Client) Notifications(cursor string) (NotificationPage, error) {
	path := "/api/v1/notifications"
	if cursor != "" {
		path += "?cursor=" + url.QueryEscape(cursor)
	}
	var out NotificationPage
	err := c.do("GET", path, nil, &out)
	return out, err
}

// Saved fetches bookmarked commits.
func (c *Client) Saved(cursor string) (Page[Commit], error) {
	path := "/api/v1/saved"
	if cursor != "" {
		path += "?cursor=" + url.QueryEscape(cursor)
	}
	var out Page[Commit]
	err := c.do("GET", path, nil, &out)
	return out, err
}

// Pastel sends a pastel on a commit.
func (c *Client) Pastel(id string) (Commit, error) {
	var out struct {
		Data Commit `json:"data"`
	}
	err := c.do("POST", apiPath("commits", id, "pasteis"), nil, &out)
	return out.Data, err
}

// Report files a moderation report.
func (c *Client) Report(id, reason, details string) error {
	body := map[string]string{"reason": reason}
	if details != "" {
		body["details"] = details
	}
	return c.do("POST", apiPath("commits", id, "reports"), body, nil)
}

func apiPath(parts ...string) string {
	var b strings.Builder
	b.WriteString("/api/v1")
	for _, p := range parts {
		b.WriteByte('/')
		b.WriteString(url.PathEscape(p))
	}
	return b.String()
}

func (c *Client) do(method, path string, payload any, dest any) error {
	if err := config.CheckTransport(c.BaseURL, c.AllowInsecureHTTP); err != nil {
		return err
	}
	if len(c.Key) != ed25519.PrivateKeySize {
		return fmt.Errorf("not signed in")
	}
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	timestamp := time.Now().Unix()
	nonce, err := randomNonce()
	if err != nil {
		return err
	}
	// Sign the URI that will actually be sent, including encoded query.
	target := req.URL.RequestURI()
	message := sign.Canonical(timestamp, nonce, method, target, string(body))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("X-Devnull-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-Devnull-Nonce", nonce)
	req.Header.Set("X-SSH-Signature", sign.Sign(c.Key, message))

	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	limited := io.LimitReader(res.Body, MaxBodyBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(raw) > MaxBodyBytes {
		return fmt.Errorf("response larger than %d bytes", MaxBodyBytes)
	}
	if res.StatusCode >= 400 {
		return parseAPIError(res.StatusCode, res.Status, raw)
	}
	if dest == nil || res.StatusCode == http.StatusNoContent || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	sanitizeWalk(dest)
	return nil
}

func parseAPIError(status int, statusText string, raw []byte) error {
	var apiErr Error
	_ = json.Unmarshal(raw, &apiErr)
	apiErr.Status = status
	if apiErr.Message == "" {
		apiErr.Message = SanitizeString(strings.TrimSpace(string(raw)))
		if apiErr.Message == "" {
			apiErr.Message = statusText
		}
	}
	apiErr.Message = clipRunes(SanitizeString(apiErr.Message), maxErrRunes)
	apiErr.Code = SanitizeString(apiErr.Code)
	return &apiErr
}

func clipRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func randomNonce() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
