package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/razajahangir/devnull-tui/internal/api"
	"github.com/razajahangir/devnull-tui/internal/auth"
	"github.com/razajahangir/devnull-tui/internal/config"
)

type errMsg struct{ err error }
type clearToastMsg struct{ id int }
type reauthMsg struct{}
type meMsg api.MeResponse
type feedMsg struct {
	sec         int
	items       []api.Commit
	suggestions []api.Commit
	next        *string
	more        bool
}
type notesMsg struct {
	page api.NotificationPage
	more bool
}
type threadMsg struct {
	page api.CommitPage
	push bool
}
type repliesMsg struct {
	id    string
	items []api.Commit
	next  *string
	more  bool
}
type profileMsg struct {
	user  api.User
	items []api.Commit
	next  *string
	push  bool
}
type profileMoreMsg struct {
	username string
	items    []api.Commit
	next     *string
}
type followedMsg struct {
	user api.User
	on   bool
}
type commitUpdatedMsg struct {
	commit api.Commit
	action string
}
type createdMsg struct {
	commit api.Commit
	reply  bool
}
type deletedMsg struct{ id string }
type pollMsg time.Time
type statusMsg struct{ commit api.Commit }
type newCountMsg struct {
	sec   int
	count int
}
type loginReadyMsg auth.Session
type callbackTickMsg time.Time
type loggedInMsg struct {
	me  api.MeResponse
	cfg config.Config
}

// Update handles Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
		m.login.Width = max(20, min(70, msg.Width-16))
		m.composer.SetWidth(max(20, min(100, msg.Width-6)))
		m.composer.SetHeight(max(3, min(10, m.bodyHeight()-9)))
		m.fix()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case clearToastMsg:
		if msg.id == m.toastID {
			m.toast = ""
			m.confirm = ""
		}
		return m, nil

	case errMsg:
		m.busy = false
		m.feeds[m.sectionFeed()].loadingMore = false
		m.notes.loadingMore = false
		m.thread.loadingMore = false
		m.profile.feed.loadingMore = false
		if needsReauth(msg.err) {
			if m.screen == screenLogin {
				return m, nil
			}
			return m.reauth()
		}
		cmd := m.flash(m.friendly(msg.err), toastErr)
		return m, cmd

	case reauthMsg:
		if m.screen == screenLogin {
			return m, nil
		}
		return m.reauth()

	case meMsg:
		m.me = msg.Data
		m.unread = msg.UnreadNotificationsCount
		return m, nil

	case feedMsg:
		m.busy = false
		f := m.feeds[msg.sec]
		f.loaded, f.loadedAt, f.loadingMore, f.next = true, time.Now(), false, msg.next
		if msg.more {
			f.items = append(f.items, msg.items...)
		} else {
			f.items, f.suggestions = msg.items, msg.suggestions
			f.cursor, f.offset, f.newCount = 0, 0, 0
		}
		m.feeds[msg.sec] = f
		m.fix()
		return m, nil

	case notesMsg:
		m.busy = false
		n := m.notes
		n.loaded, n.loadingMore, n.next = true, false, msg.page.Meta.NextCursor
		if msg.more {
			n.items = append(n.items, msg.page.Data...)
		} else {
			n.items, n.readAt = msg.page.Data, msg.page.ReadAt
			n.cursor, n.offset = 0, 0
		}
		m.notes = n
		m.unread = 0
		m.fix()
		return m, nil

	case threadMsg:
		m.busy = false
		if msg.push {
			m.push()
		}
		t := threadState{commit: msg.page.Data, ancestors: msg.page.Ancestors}
		t.cursor = t.main()
		m.thread = t
		m.screen = screenThread
		m.fix()
		return m, m.loadReplies(t.commit.ID, nil)

	case repliesMsg:
		if msg.id != m.thread.commit.ID {
			return m, nil
		}
		m.thread.repliesLoaded, m.thread.loadingMore, m.thread.next = true, false, msg.next
		if msg.more {
			m.thread.replies = append(m.thread.replies, msg.items...)
		} else {
			m.thread.replies = msg.items
		}
		m.fix()
		return m, nil

	case profileMsg:
		m.busy = false
		if msg.push {
			m.push()
		}
		m.profile = profileState{user: msg.user, feed: feed{items: msg.items, next: msg.next, loaded: true, loadedAt: time.Now()}}
		m.screen = screenProfile
		m.fix()
		return m, nil

	case profileMoreMsg:
		if msg.username != m.profile.user.Username {
			return m, nil
		}
		m.profile.feed.items = append(m.profile.feed.items, msg.items...)
		m.profile.feed.next, m.profile.feed.loadingMore = msg.next, false
		m.fix()
		return m, nil

	case followedMsg:
		m.busy = false
		if m.profile.user.ID == msg.user.ID {
			prev := m.profile.user
			m.profile.user = msg.user
			m.profile.user.Activity = prev.Activity
			m.profile.user.Streaks = prev.Streaks
			m.profile.user.Achievements = prev.Achievements
		}
		m.feeds[secFollowing].loaded = false
		text := "Unfollowed @" + msg.user.Username
		if msg.on {
			text = "Following @" + msg.user.Username + " · their commits now show up in 1 Following"
		}
		cmd := m.flash(text, toastOK)
		return m, cmd

	case commitUpdatedMsg:
		m.busy = false
		m.applyCommit(msg.commit, msg.action)
		m.fix()
		var text string
		switch {
		case msg.action == "star" && msg.commit.Starred:
			text = "Starred"
		case msg.action == "star":
			text = "Unstarred"
		case msg.commit.IsSaved:
			text = "Saved · find it under 4 Saved"
		default:
			text = "Removed from Saved"
		}
		cmd := m.flash(text, toastOK)
		return m, cmd

	case createdMsg:
		m.busy = false
		m.composing = false
		m.composer.Blur()
		m.feeds[secFollowing].loaded = false
		m.feeds[secLatest].loaded = false
		text := "Committed " + msg.commit.ShortHash
		if msg.commit.Status == "pending" {
			text += " · pending review, only you can see it until it's approved"
		}
		cmds := []tea.Cmd{m.flash(text, toastOK)}
		switch {
		case m.screen == screenThread:
			cmds = append(cmds, m.loadThread(m.thread.commit.ID, false))
		case m.screen == screenSection && (m.section == secFollowing || m.section == secLatest):
			m.busy = true
			cmds = append(cmds, m.loadSection(m.section, false))
		}
		return m, tea.Batch(cmds...)

	case deletedMsg:
		m.busy = false
		m.removeCommit(msg.id)
		var cmds []tea.Cmd
		if m.screen == screenThread && m.thread.commit.ID == msg.id {
			m.pop()
		} else if m.screen == screenThread {
			cmds = append(cmds, m.loadThread(m.thread.commit.ID, false))
		}
		m.fix()
		cmds = append(cmds, m.flash("Deleted", toastOK))
		return m, tea.Batch(cmds...)

	case pollMsg:
		cmds := []tea.Cmd{m.pollCmd()}
		if m.screen == screenLogin {
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, m.loadMe(true))
		for _, sec := range []int{secFollowing, secLatest} {
			if f := m.feeds[sec]; f.loaded && len(f.items) > 0 {
				cmds = append(cmds, m.checkNew(sec, f.items[0].ID))
			}
		}
		for _, id := range m.pendingIDs() {
			cmds = append(cmds, m.checkStatus(id))
		}
		return m, tea.Batch(cmds...)

	case statusMsg:
		m.applyCommit(msg.commit, "status")
		m.fix()
		if msg.commit.Status == "approved" {
			return m, m.flash("Your commit "+msg.commit.ShortHash+" was approved and is now public", toastOK)
		}
		return m, nil

	case newCountMsg:
		m.feeds[msg.sec].newCount = msg.count
		m.fix()
		if msg.count > 0 && m.cfg.AutoLoadNew && m.screen == screenSection && m.section == msg.sec {
			if m.feeds[msg.sec].cursor == 0 {
				m.busy, m.busyLabel = true, ""
				return m, m.loadSection(msg.sec, false)
			}
		}
		return m, nil

	case loginReadyMsg:
		m.busy = false
		m.session = auth.Session(msg)
		_ = auth.RegisterProtocolHandler()
		if err := auth.AllowedBrowserURL(m.cfg.URL, m.session.AuthorizeURL, m.cfg.AllowInsecureHTTP); err == nil {
			_ = auth.OpenBrowser(m.session.AuthorizeURL)
		}
		return m, waitCallback()

	case callbackTickMsg:
		if m.screen != screenLogin || m.session.State == "" {
			return m, nil
		}
		if raw, ok := auth.ReadCallback(); ok {
			m.login.SetValue(raw)
			m.busy, m.busyLabel = true, "Signing in…"
			return m, m.redeemLogin()
		}
		return m, waitCallback()

	case loggedInMsg:
		m.cfg = msg.cfg
		m.client = api.NewWithOptions(msg.cfg.URL, m.session.Private, msg.cfg.AllowInsecureHTTP)
		m.me = msg.me.Data
		m.unread = msg.me.UnreadNotificationsCount
		m.feeds = [4]feed{}
		m.notes = notesState{}
		m.stack = nil
		m.section = secFollowing
		m.screen = screenSection
		m.busy, m.busyLabel = true, ""
		m.login.SetValue("")
		cmd := m.flash("Welcome, @"+m.me.Username+"! Press ? any time to see every key.", toastOK)
		return m, tea.Batch(cmd, m.loadSection(secFollowing, false))

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		if m.screen == screenLogin || m.composing || m.showHelp || msg.Action != tea.MouseActionPress {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			return m.move(-1)
		case tea.MouseButtonWheelDown:
			return m.move(1)
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch {
	case m.screen == screenLogin:
		m.login, cmd = m.login.Update(msg)
	case m.composing:
		m.composer, cmd = m.composer.Update(msg)
	}
	return m, cmd
}

func (m Model) sectionFeed() int {
	if m.section < len(m.feeds) {
		return m.section
	}
	return 0
}

func (m Model) reauth() (tea.Model, tea.Cmd) {
	_ = config.ForgetKey(m.cfg.URL)
	m.screen = screenLogin
	m.stack = nil
	m.me = api.Me{}
	m.busy = true
	m.busyLabel = "Preparing sign-in…"
	cmd := m.flash("This terminal isn't signed in on "+m.host()+" yet. Approve it in the browser.", toastInfo)
	return m, tea.Batch(cmd, m.startLogin())
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m.quit()
	}
	switch {
	case m.screen == screenLogin:
		return m.loginKeys(msg)
	case m.composing:
		return m.composeKeys(msg)
	case m.showHelp:
		if key.Matches(msg, m.keys.Help, m.keys.Back, m.keys.Quit) {
			m.showHelp = false
		}
		return m, nil
	}

	k := m.keys
	if m.confirm != "" {
		pending := m.confirm
		m.confirm, m.toast = "", ""
		switch {
		case pending == "delete" && key.Matches(msg, k.Delete):
			m.busy, m.busyLabel = true, "Deleting…"
			return m, m.deleteCommit(m.pendingDelete)
		case pending == "signout" && key.Matches(msg, k.SignOut):
			return m.signOut()
		case key.Matches(msg, k.Back):
			cmd := m.flash("Cancelled", toastInfo)
			return m, cmd
		}
	}

	switch {
	case key.Matches(msg, k.Help):
		m.showHelp = true
		return m, nil
	case key.Matches(msg, k.Quit):
		if len(m.stack) > 0 {
			m.pop()
			m.fix()
			return m, nil
		}
		return m.quit()
	case key.Matches(msg, k.SignOut):
		m.confirm = "signout"
		cmd := m.flash("Sign out of this terminal? Press ctrl+l again to confirm, any other key to cancel.", toastInfo)
		return m, cmd
	case key.Matches(msg, k.NextTab):
		return m.gotoSection((m.section + 1) % len(sections))
	case key.Matches(msg, k.PrevTab):
		return m.gotoSection((m.section + len(sections) - 1) % len(sections))
	case key.Matches(msg, k.Compose):
		return m.openCompose(nil)
	case key.Matches(msg, k.Refresh):
		return m.refresh()
	case key.Matches(msg, k.Me):
		if m.me.Username == "" {
			return m, nil
		}
		m.busy, m.busyLabel = true, ""
		return m, m.loadProfile(m.me.Username, true)
	case key.Matches(msg, k.Up):
		return m.move(-1)
	case key.Matches(msg, k.Down):
		return m.move(1)
	case key.Matches(msg, k.PageUp):
		return m.move(-5)
	case key.Matches(msg, k.PageDown):
		return m.move(5)
	case key.Matches(msg, k.Top):
		return m.move(-1 << 20)
	case key.Matches(msg, k.Bottom):
		return m.move(1 << 20)
	case key.Matches(msg, k.Open):
		return m.open()
	case key.Matches(msg, k.Back):
		return m.back()
	case key.Matches(msg, k.Star):
		return m.act("star")
	case key.Matches(msg, k.Save):
		return m.act("save")
	case key.Matches(msg, k.Reply):
		return m.act("reply")
	case key.Matches(msg, k.Profile):
		return m.act("profile")
	case key.Matches(msg, k.Browser):
		return m.openInBrowser()
	case key.Matches(msg, k.Delete):
		return m.act("delete")
	case key.Matches(msg, k.Follow):
		return m.follow()
	}
	for i, b := range k.Sections {
		if key.Matches(msg, b) {
			return m.gotoSection(i)
		}
	}
	return m, nil
}

func (m Model) loginKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.quit()
	case "ctrl+o":
		if m.session.AuthorizeURL != "" {
			if err := auth.AllowedBrowserURL(m.cfg.URL, m.session.AuthorizeURL, m.cfg.AllowInsecureHTTP); err == nil {
				_ = auth.OpenBrowser(m.session.AuthorizeURL)
			}
		}
		cmd := m.flash("Opened the approval page in your browser again", toastInfo)
		return m, cmd
	case "enter":
		if strings.TrimSpace(m.login.Value()) == "" {
			cmd := m.flash("Approve in the browser first, or paste the devnull:// address here.", toastInfo)
			return m, cmd
		}
		m.busy, m.busyLabel = true, "Signing in…"
		return m, m.redeemLogin()
	}
	var cmd tea.Cmd
	m.login, cmd = m.login.Update(msg)
	return m, cmd
}

func (m Model) composeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Send):
		if m.busy {
			return m, nil
		}
		body := strings.TrimSpace(m.composer.Value())
		n := utf8.RuneCountInString(body)
		if n < 3 {
			cmd := m.flash("Write at least 3 characters before sending.", toastErr)
			return m, cmd
		}
		if n > 4000 {
			cmd := m.flash("That's over 4000 characters. Trim it a little.", toastErr)
			return m, cmd
		}
		m.busy, m.busyLabel = true, "Sending…"
		return m, m.submit(body, m.composeTarget)
	case msg.String() == "esc":
		if strings.TrimSpace(m.composer.Value()) != "" && m.confirm != "discard" {
			m.confirm = "discard"
			cmd := m.flash("Discard this draft? Press esc again to discard, or keep typing.", toastInfo)
			return m, cmd
		}
		m.composing = false
		m.confirm, m.toast = "", ""
		m.composer.Blur()
		return m, nil
	}
	if m.confirm == "discard" {
		m.confirm, m.toast = "", ""
	}
	var cmd tea.Cmd
	m.composer, cmd = m.composer.Update(msg)
	return m, cmd
}

func (m Model) gotoSection(i int) (tea.Model, tea.Cmd) {
	m.section = i
	m.screen = screenSection
	m.stack = nil
	if i == secNotes {
		m.busy, m.busyLabel = true, ""
		return m, m.loadNotes(false)
	}
	f := m.feeds[i]
	stale := !f.loaded
	if m.cfg.RefreshOnSwitch && f.loaded {
		if f.newCount > 0 {
			stale = true
		} else if m.cfg.PollInterval > 0 && !f.loadedAt.IsZero() && time.Since(f.loadedAt) > m.cfg.PollInterval {
			stale = true
		}
	}
	if stale {
		m.busy, m.busyLabel = true, ""
		return m, m.loadSection(i, false)
	}
	m.fix()
	return m, nil
}

func (m Model) refresh() (tea.Model, tea.Cmd) {
	m.busy, m.busyLabel = true, "Refreshing…"
	for i := range m.feeds {
		if m.screen != screenSection || i != m.section {
			m.feeds[i].loaded = false
		}
	}
	switch m.screen {
	case screenThread:
		return m, m.loadThread(m.thread.commit.ID, false)
	case screenProfile:
		return m, m.loadProfile(m.profile.user.Username, false)
	}
	if m.section == secNotes {
		return m, m.loadNotes(false)
	}
	return m, m.loadSection(m.section, false)
}

func (m Model) move(delta int) (tea.Model, tea.Cmd) {
	n, cur, _ := m.refs()
	if cur == nil || n == 0 {
		return m, nil
	}
	*cur = min(max(*cur+delta, 0), n-1)
	m.fix()
	cmd := m.maybeMore()
	return m, cmd
}

func (m *Model) maybeMore() tea.Cmd {
	n, cur, _ := m.refs()
	if cur == nil || *cur < n-3 {
		return nil
	}
	switch m.screen {
	case screenSection:
		if m.section == secNotes {
			if m.notes.next != nil && !m.notes.loadingMore {
				m.notes.loadingMore = true
				return m.loadNotes(true)
			}
			return nil
		}
		f := &m.feeds[m.section]
		if len(f.items) > 0 && f.next != nil && !f.loadingMore {
			f.loadingMore = true
			return m.loadSection(m.section, true)
		}
	case screenThread:
		if m.thread.next != nil && !m.thread.loadingMore {
			m.thread.loadingMore = true
			return m.loadReplies(m.thread.commit.ID, m.thread.next)
		}
	case screenProfile:
		if f := &m.profile.feed; f.next != nil && !f.loadingMore {
			f.loadingMore = true
			return m.loadProfileMore(m.profile.user.Username, *f.next)
		}
	}
	return nil
}

func (m Model) open() (tea.Model, tea.Cmd) {
	if m.screen == screenSection && m.section == secNotes {
		if m.notes.cursor >= len(m.notes.items) {
			return m, nil
		}
		n := m.notes.items[m.notes.cursor]
		switch {
		case n.Commit != nil && n.Commit.Author != nil:
			m.busy, m.busyLabel = true, ""
			return m, m.loadThread(n.Commit.ID, true)
		case n.Actor != nil:
			m.busy, m.busyLabel = true, ""
			return m, m.loadProfile(n.Actor.Username, true)
		}
		return m, nil
	}
	if m.screen == screenThread && m.thread.cursor == m.thread.main() {
		cmd := m.flash("You're reading this commit. r replies, p opens the author, ↓ goes to the replies.", toastInfo)
		return m, cmd
	}
	c, ok := m.selected()
	if !ok {
		return m, nil
	}
	m.busy, m.busyLabel = true, ""
	return m, m.loadThread(c.ID, true)
}

func (m Model) back() (tea.Model, tea.Cmd) {
	if len(m.stack) > 0 || m.screen != screenSection {
		m.pop()
		m.fix()
		return m, nil
	}
	cmd := m.flash("You're at the top. tab or 1–5 switch sections, q quits.", toastInfo)
	return m, cmd
}

func (m Model) act(action string) (tea.Model, tea.Cmd) {
	c, ok := m.selected()
	if !ok {
		cmd := m.flash("Select a commit first (↑/↓).", toastInfo)
		return m, cmd
	}
	switch action {
	case "star":
		m.busy, m.busyLabel = true, ""
		return m, m.toggleStar(c)
	case "save":
		m.busy, m.busyLabel = true, ""
		return m, m.toggleSave(c)
	case "reply":
		return m.openCompose(&c)
	case "profile":
		if m.screen == screenProfile && m.profile.user.ID == c.Author.ID {
			cmd := m.flash("You're already on @"+c.Author.Username+"'s profile.", toastInfo)
			return m, cmd
		}
		m.busy, m.busyLabel = true, ""
		return m, m.loadProfile(c.Author.Username, true)
	case "delete":
		if !m.isMine(c) {
			cmd := m.flash("You can only delete your own commits.", toastInfo)
			return m, cmd
		}
		m.confirm = "delete"
		m.pendingDelete = c.ID
		cmd := m.flash(fmt.Sprintf("Delete commit %s? Press d again to confirm, any other key to cancel.", c.ShortHash), toastErr)
		return m, cmd
	}
	return m, nil
}

func (m Model) follow() (tea.Model, tea.Cmd) {
	if m.screen != screenProfile {
		cmd := m.flash("Open someone's profile first (p on a commit), then press f.", toastInfo)
		return m, cmd
	}
	u := m.profile.user
	if u.ID == m.me.ID {
		cmd := m.flash("That's you.", toastInfo)
		return m, cmd
	}
	on := u.IsFollowed == nil || !*u.IsFollowed
	m.busy, m.busyLabel = true, ""
	return m, m.setFollow(u.Username, on)
}

func (m Model) openInBrowser() (tea.Model, tea.Cmd) {
	target := ""
	kind := "page"
	if c, ok := m.selected(); ok {
		target = commitPageURL(m.cfg.URL, c)
		kind = "commit"
	} else if m.screen == screenProfile {
		target = m.profile.user.HTMLURL
		kind = "profile"
	} else if m.screen == screenSection && m.section == secNotes && m.notes.cursor < len(m.notes.items) {
		n := m.notes.items[m.notes.cursor]
		if n.Commit != nil {
			target = commitPageURL(m.cfg.URL, *n.Commit)
			kind = "commit"
		} else if n.Actor != nil {
			target = n.Actor.HTMLURL
			kind = "profile"
		}
	}
	if target == "" {
		cmd := m.flash("Nothing to open in the browser. Highlight a commit, then press o.", toastInfo)
		return m, cmd
	}
	if err := auth.AllowedBrowserURL(m.cfg.URL, target, m.cfg.AllowInsecureHTTP); err != nil {
		cmd := m.flash(err.Error(), toastErr)
		return m, cmd
	}
	if err := auth.OpenBrowser(target); err != nil {
		cmd := m.flash("Couldn't open the browser · "+target, toastErr)
		return m, cmd
	}
	cmd := m.flash("Opened this "+kind+" in your browser", toastOK)
	return m, cmd
}

func (m Model) openCompose(target *api.Commit) (tea.Model, tea.Cmd) {
	m.composing = true
	m.composeTarget = target
	m.confirm, m.toast = "", ""
	m.composer.Reset()
	m.composer.SetHeight(max(3, min(10, m.bodyHeight()-9)))
	m.composer.Focus()
	return m, textarea.Blink
}

func (m Model) signOut() (tea.Model, tea.Cmd) {
	_ = config.ForgetKey(m.cfg.URL)
	m.screen = screenLogin
	m.stack = nil
	m.me = api.Me{}
	m.feeds = [4]feed{}
	m.notes = notesState{}
	m.busy, m.busyLabel = true, "Preparing sign-in…"
	cmd := m.flash("Signed out here. To revoke this device completely, use Settings → Devices on the web.", toastInfo)
	return m, tea.Batch(cmd, m.startLogin())
}
