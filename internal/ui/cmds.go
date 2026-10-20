package ui

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/razajahangir/devnull-tui/internal/api"
	"github.com/razajahangir/devnull-tui/internal/auth"
	"github.com/razajahangir/devnull-tui/internal/config"
)

func (m Model) pollCmd() tea.Cmd {
	d := m.cfg.PollInterval
	if d < config.MinPollInterval {
		d = config.DefaultPollInterval
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return pollMsg(t) })
}

func waitCallback() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg { return callbackTickMsg(t) })
}

func needsReauth(err error) bool {
	var e *api.Error
	return errors.As(err, &e) && e.Status == 401 && e.Code == "device_not_approved"
}

func (m Model) friendly(err error) string {
	var e *api.Error
	if errors.As(err, &e) {
		switch e.Code {
		case "rate_limited":
			return "Slow down: too many requests. Try again in a minute."
		case "invalid_signature":
			return "The server rejected this device's signature. Check your system clock, or press ctrl+l to sign in again."
		case "invalid_grant":
			return "That approval expired or was already used. Press ctrl+o to approve again."
		case "commit_unavailable":
			return "That commit isn't available anymore."
		case "user_blocked", "user_unavailable":
			return "This profile isn't available."
		case "own_commit":
			return "You can't do that to your own commit."
		}
		switch {
		case e.Status == 404:
			return "Not found. It may have been deleted."
		case e.Status == 422 && e.Message != "":
			return e.Message
		case e.Status >= 500:
			return fmt.Sprintf("%s had a problem (HTTP %d). Try again in a moment.", m.host(), e.Status)
		}
		return e.Error()
	}
	var ue *url.Error
	var ne net.Error
	if errors.As(err, &ue) || errors.As(err, &ne) {
		return "Can't reach " + m.host() + ". Is it running, and are you online?"
	}
	return err.Error()
}

func (m Model) startLogin() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		session, err := auth.Start(cfg.URL, cfg.DeviceName)
		if err != nil {
			return errMsg{err}
		}
		return loginReadyMsg(session)
	}
}

func (m Model) redeemLogin() tea.Cmd {
	input := m.login.Value()
	session := m.session
	cfg := m.cfg
	return func() tea.Msg {
		code, err := auth.ParseCallback(input, session.State)
		if err != nil {
			return errMsg{err}
		}
		me, err := api.NewWithOptions(cfg.URL, session.Private, cfg.AllowInsecureHTTP).RedeemDevice(code, session.Verifier)
		if err != nil {
			return errMsg{err}
		}
		if err := config.SaveKey(cfg.URL, session.Private); err != nil {
			return errMsg{err}
		}
		return loggedInMsg{me: me, cfg: cfg}
	}
}

func (m Model) loadMe(silent bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		me, err := client.Me()
		if err != nil {
			if needsReauth(err) {
				return reauthMsg{}
			}
			if silent {
				return nil
			}
			return errMsg{err}
		}
		return meMsg(me)
	}
}

func (m Model) loadSection(sec int, more bool) tea.Cmd {
	client := m.client
	cursor := ""
	if more && m.feeds[sec].next != nil {
		cursor = *m.feeds[sec].next
	}
	return func() tea.Msg {
		if sec == secSaved {
			page, err := client.Saved(cursor)
			if err != nil {
				return errMsg{err}
			}
			return feedMsg{sec: sec, items: page.Data, next: page.Meta.NextCursor, more: more}
		}
		page, err := client.Log(sections[sec].kind, cursor, "")
		if err != nil {
			return errMsg{err}
		}
		return feedMsg{sec: sec, items: page.Data, suggestions: page.Suggestions, next: page.Meta.NextCursor, more: more}
	}
}

func (m Model) checkNew(sec int, since string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		page, err := client.Log(sections[sec].kind, "", since)
		if err != nil {
			return nil
		}
		return newCountMsg{sec: sec, count: page.NewCommitsCount}
	}
}

func (m Model) checkStatus(id string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		page, err := client.Commit(id)
		if err != nil || page.Data.Status == "" || page.Data.Status == "pending" {
			return nil
		}
		return statusMsg{commit: page.Data}
	}
}

func (m Model) loadNotes(more bool) tea.Cmd {
	client := m.client
	cursor := ""
	if more && m.notes.next != nil {
		cursor = *m.notes.next
	}
	return func() tea.Msg {
		page, err := client.Notifications(cursor)
		if err != nil {
			return errMsg{err}
		}
		return notesMsg{page: page, more: more}
	}
}

func (m Model) loadThread(id string, push bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		page, err := client.Commit(id)
		if err != nil {
			return errMsg{err}
		}
		return threadMsg{page: page, push: push}
	}
}

func (m Model) loadReplies(id string, cursor *string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		c := ""
		if cursor != nil {
			c = *cursor
		}
		page, err := client.Replies(id, c, false)
		if err != nil {
			return errMsg{err}
		}
		return repliesMsg{id: id, items: page.Data, next: page.Meta.NextCursor, more: cursor != nil}
	}
}

func (m Model) loadProfile(username string, push bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		user, err := client.User(username)
		if err != nil {
			return errMsg{err}
		}
		_ = client.EnrichUser(&user)
		page, err := client.UserCommits(username, "")
		if err != nil {
			return errMsg{err}
		}
		return profileMsg{user: user, items: page.Data, next: page.Meta.NextCursor, push: push}
	}
}

func (m Model) loadProfileMore(username, cursor string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		page, err := client.UserCommits(username, cursor)
		if err != nil {
			return errMsg{err}
		}
		return profileMoreMsg{username: username, items: page.Data, next: page.Meta.NextCursor}
	}
}

func (m Model) submit(body string, target *api.Commit) tea.Cmd {
	client := m.client
	var parent *string
	if target != nil {
		id := target.ID
		parent = &id
	}
	return func() tea.Msg {
		commit, err := client.CreateCommit(body, parent, nil)
		if err != nil {
			return errMsg{err}
		}
		return createdMsg{commit: commit, reply: parent != nil}
	}
}

func (m Model) toggleStar(c api.Commit) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		commit, err := client.Star(c.ID, !c.Starred)
		if err != nil {
			return errMsg{err}
		}
		return commitUpdatedMsg{commit: commit, action: "star"}
	}
}

func (m Model) toggleSave(c api.Commit) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		commit, err := client.Bookmark(c.ID, !c.IsSaved)
		if err != nil {
			return errMsg{err}
		}
		return commitUpdatedMsg{commit: commit, action: "save"}
	}
}

func (m Model) setFollow(username string, on bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if _, err := client.Follow(username, on); err != nil {
			return errMsg{err}
		}
		user, err := client.User(username)
		if err != nil {
			return errMsg{err}
		}
		return followedMsg{user: user, on: on}
	}
}

func (m Model) deleteCommit(id string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if err := client.DeleteCommit(id); err != nil {
			return errMsg{err}
		}
		return deletedMsg{id: id}
	}
}

func (m Model) quit() (tea.Model, tea.Cmd) {
	return m, tea.Quit
}
