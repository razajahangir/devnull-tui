package ui

import (
	"crypto/ed25519"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/razajahangir/devnull-tui/internal/api"
	"github.com/razajahangir/devnull-tui/internal/config"
)

func fixture(t *testing.T) Model {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(nil)
	cfg := config.Config{
		URL:             "http://127.0.0.1:1",
		DeviceName:      "test",
		PollInterval:    30 * time.Second,
		RefreshOnSwitch: true,
	}
	m := New(cfg, api.New(cfg.URL, priv), true)
	me := api.Me{User: api.User{ID: "u1", Username: "me"}}
	alice := &api.User{ID: "u2", Username: "alice", Name: "Alice"}
	now := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
	commits := []api.Commit{
		{ID: "c1", ShortHash: "abc1234", Body: "hello world", CreatedAt: now, Author: alice, StarsCount: 2, Starred: true, Images: []api.CommitImage{{ID: "i1"}}},
		{ID: "c2", ShortHash: "def5678", Body: strings.Repeat("a long line of text ", 40), CreatedAt: now, Author: &me.User, Status: "pending"},
		{ID: "c3", ShortHash: "0000000"},
	}
	var model tea.Model = m
	step := func(msg tea.Msg) { model, _ = model.Update(msg) }
	step(tea.WindowSizeMsg{Width: 100, Height: 30})
	step(meMsg(api.MeResponse{Data: me, UnreadNotificationsCount: 3}))
	step(feedMsg{sec: secFollowing, items: commits})
	step(feedMsg{sec: secLatest, items: nil})
	return model.(Model)
}

func TestViewsRenderAtManySizes(t *testing.T) {
	m := fixture(t)
	var model tea.Model = m
	keys := []string{"j", "j", "k", "?", "esc", "2", "4", "5", "1", "c", "esc", "G", "g"}
	for _, size := range [][2]int{{40, 10}, {60, 16}, {100, 30}, {200, 60}} {
		model, _ = model.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, k := range keys {
			var msg tea.KeyMsg
			switch k {
			case "esc":
				msg = tea.KeyMsg{Type: tea.KeyEsc}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
			}
			model, _ = model.Update(msg)
			out := model.View()
			if out == "" {
				t.Fatalf("empty view after %q at %v", k, size)
			}
			if h := strings.Count(out, "\n") + 1; h > size[1] {
				t.Fatalf("view is %d lines tall, window is %d (after %q at %v)", h, size[1], k, size)
			}
		}
	}
}

func TestApprovalClearsPendingBadge(t *testing.T) {
	m := fixture(t)
	if ids := m.pendingIDs(); len(ids) != 1 || ids[0] != "c2" {
		t.Fatalf("pendingIDs = %v, want [c2]", ids)
	}
	if !strings.Contains(m.View(), "pending review") {
		t.Fatal("expected the pending badge before approval")
	}
	var model tea.Model = m
	model, _ = model.Update(statusMsg{commit: api.Commit{ID: "c2", ShortHash: "def5678", Status: "approved"}})
	m = model.(Model)
	if strings.Contains(m.View(), "pending review") {
		t.Fatal("pending badge still shown after approval")
	}
	if ids := m.pendingIDs(); len(ids) != 0 {
		t.Fatalf("pendingIDs = %v after approval, want none", ids)
	}
}

func TestRefreshMarksOtherTabsStale(t *testing.T) {
	var model tea.Model = fixture(t)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	m := model.(Model)
	if !m.feeds[secFollowing].loaded {
		t.Fatal("the current tab should keep showing its items while it reloads")
	}
	if m.feeds[secLatest].loaded {
		t.Fatal("Latest should be marked stale after R")
	}
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if cmd == nil {
		t.Fatal("switching to a stale tab should reload it")
	}
}

func TestAutoLoadNewAtTop(t *testing.T) {
	m := fixture(t)
	m.cfg.AutoLoadNew = true
	m.feeds[secFollowing].cursor = 0
	var model tea.Model = m
	_, cmd := model.Update(newCountMsg{sec: secFollowing, count: 3})
	if cmd == nil {
		t.Fatal("expected a reload when auto_load_new is on and cursor is at the top")
	}
}

func TestRefreshOnSwitchWhenNewCount(t *testing.T) {
	m := fixture(t)
	m.cfg.RefreshOnSwitch = true
	m.feeds[secLatest].loaded = true
	m.feeds[secLatest].loadedAt = time.Now()
	m.feeds[secLatest].newCount = 2
	_, cmd := m.gotoSection(secLatest)
	if cmd == nil {
		t.Fatal("expected reload when switching to a tab with new commits")
	}
}

func TestProfileHeaderShowsAuthorDetails(t *testing.T) {
	m := fixture(t)
	m.screen = screenProfile
	m.profile.user = api.User{
		ID:             "u2",
		Username:       "alice",
		Name:           "Alice Example",
		Bio:            "Builds tools for people.",
		Company:        "Acme",
		Location:       "Karachi",
		Website:        "https://alice.dev",
		GitHubURL:      "https://github.com/alice",
		CreatedAt:      time.Now().Add(-400 * 24 * time.Hour).Format(time.RFC3339),
		CommitsCount:   12,
		FollowersCount: 3,
		FollowingCount: 8,
		Streaks:        &api.UserStreaks{Longest: 3, Current: 2},
		Activity: &api.UserActivity{Scale: "week", Buckets: []api.UserActivityBucket{
			{Date: "2026-10-18", Count: 1},
			{Date: "2026-10-19", Count: 4},
		}},
		Achievements: &api.UserAchievements{
			Level: 4, Rank: "maintainer", XP: 759, NextLevelXP: 1000, LevelProgress: 60,
			Ranks:  []api.UserRank{{Rank: "wheel", Level: 5, Achieved: false}},
			Badges: []api.UserBadge{{Name: "init", Unlocked: true, Progress: 1, Target: 1}},
		},
	}
	m.profile.feed.loaded = true
	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 42})
	out := model.View()
	for _, want := range []string{
		"PROFILE", "Alice Example", "@alice", "Builds tools",
		"company", "Acme", "location", "Karachi", "website", "alice.dev",
		"github", "github.com/alice", "joined", "12", "follower", "following",
		"+ Follow", "alice's commits",
		"Commit activity", "longest streak", "Lv. 4", "maintainer", "759 XP", "init",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("profile missing %q\n%s", want, out)
		}
	}
}

func TestOwnProfileIsLabeled(t *testing.T) {
	m := fixture(t)
	m.screen = screenProfile
	m.profile.user = m.me.User
	m.profile.user.Name = "Me"
	m.profile.feed.loaded = true
	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 36})
	out := model.View()
	for _, want := range []string{"YOUR PROFILE", "you", "This is you", "Your commits"} {
		if !strings.Contains(out, want) {
			t.Errorf("own profile missing %q\n%s", want, out)
		}
	}
}

func TestCommitStatsSitUnderBody(t *testing.T) {
	m := fixture(t)
	out := m.View()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "@alice") && (strings.Contains(line, "☆") || strings.Contains(line, "★")) {
			t.Fatalf("stars still on the author line: %q", line)
		}
	}
	if !strings.Contains(out, "hello world") {
		t.Fatal("missing commit body")
	}
	if !strings.Contains(out, "★ 2") {
		t.Fatalf("missing star count under the commit:\n%s", out)
	}
	if !strings.Contains(out, "↩ 0") {
		t.Fatalf("missing reply count under the commit:\n%s", out)
	}
	if !strings.Contains(out, "🖼") || !strings.Contains(out, "1 image") {
		t.Fatalf("missing image marker:\n%s", out)
	}
}

func TestCommitPageURL(t *testing.T) {
	alice := &api.User{Username: "alice"}
	got := commitPageURL("https://dev-null.sh", api.Commit{ID: "abc", Author: alice, HTMLURL: "https://dev-null.sh/@alice/commits/abc"})
	if got != "https://dev-null.sh/@alice/commits/abc" {
		t.Fatal(got)
	}
	got = commitPageURL("https://dev-null.sh", api.Commit{ID: "01a1", Author: alice})
	if got != "https://dev-null.sh/@alice/commits/01a1" {
		t.Fatal(got)
	}
	if commitPageURL("https://dev-null.sh", api.Commit{}) != "" {
		t.Fatal("empty commit should have no url")
	}
}

func TestFooterShowsContextualKeys(t *testing.T) {
	m := fixture(t)
	out := m.View()
	for _, want := range []string{"Following", "unstar", "reply", "all keys"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}
}
