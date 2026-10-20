package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestEnrichUserLoadsAchievements(t *testing.T) {
	setInertiaVersion("")
	page := map[string]any{
		"component": "user/show",
		"props": map[string]any{
			"activity": map[string]any{
				"scale": "week",
				"buckets": []map[string]any{
					{"date": "2026-10-18", "count": 2},
					{"date": "2026-10-19", "count": 0},
				},
			},
			"streaks": map[string]any{"longest": 3, "current": 2},
			"achievements": map[string]any{
				"level": 4, "rank": "maintainer", "xp": 759,
				"next_level_xp": 1000, "level_progress": 60,
				"badges": []map[string]any{
					{"id": "init", "name": "init", "unlocked": true, "progress": 1, "target": 1},
				},
			},
		},
	}
	raw, _ := json.Marshal(page)
	c, srv, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Inertia") != "true" {
			t.Fatal("missing X-Inertia")
		}
		if r.Header.Get("X-SSH-Signature") != "" {
			t.Fatal("must not sign the public profile page")
		}
		if r.Header.Get("X-Inertia-Version") == "" {
			w.Header().Set("X-Inertia-Version", "abc")
			w.WriteHeader(http.StatusConflict)
			return
		}
		if r.URL.Path != "/@alice" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	})
	u := User{Username: "alice", HTMLURL: srv.URL + "/@alice"}
	if err := c.EnrichUser(&u); err != nil {
		t.Fatal(err)
	}
	if u.Achievements == nil || u.Achievements.Rank != "maintainer" || u.Achievements.XP != 759 {
		t.Fatalf("achievements %+v", u.Achievements)
	}
	if u.Streaks == nil || u.Streaks.Current != 2 {
		t.Fatalf("streaks %+v", u.Streaks)
	}
	if u.Activity == nil || len(u.Activity.Buckets) != 2 {
		t.Fatalf("activity %+v", u.Activity)
	}
}

func TestEnrichUserRefusesOffInstance(t *testing.T) {
	c, _, _ := testClient(t, func(http.ResponseWriter, *http.Request) {})
	u := User{Username: "alice", HTMLURL: "https://evil.example/@alice"}
	if err := c.EnrichUser(&u); err == nil {
		t.Fatal("expected refusal")
	}
}
