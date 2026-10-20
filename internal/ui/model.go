package ui

import (
	"net/url"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/razajahangir/devnull-tui/internal/api"
	"github.com/razajahangir/devnull-tui/internal/auth"
	"github.com/razajahangir/devnull-tui/internal/config"
)

type screen int

const (
	screenLogin screen = iota
	screenSection
	screenThread
	screenProfile
)

const (
	secFollowing = iota
	secLatest
	secTrending
	secSaved
	secNotes
)

type section struct {
	key, label, short, kind, blurb string
}

var sections = []section{
	{"1", "Following", "Fol", "following", "commits from people you follow"},
	{"2", "Latest", "New", "latest", "the newest commits from everyone"},
	{"3", "Trending", "Hot", "trending", "what people are engaging with right now"},
	{"4", "Saved", "Sav", "saved", "commits you saved with b · only you can see these"},
	{"5", "Notifications", "Ntf", "notifications", "replies, stars, mentions and new followers"},
}

type feed struct {
	items       []api.Commit
	suggestions []api.Commit
	cursor      int
	offset      int
	next        *string
	loaded      bool
	loadedAt    time.Time
	loadingMore bool
	newCount    int
}

func (f feed) list() []api.Commit {
	if len(f.items) == 0 {
		return f.suggestions
	}
	return f.items
}

type notesState struct {
	items       []api.Notification
	readAt      string
	cursor      int
	offset      int
	next        *string
	loaded      bool
	loadingMore bool
}

type threadState struct {
	commit        api.Commit
	ancestors     []api.Commit
	replies       []api.Commit
	cursor        int
	offset        int
	next          *string
	repliesLoaded bool
	loadingMore   bool
}

func (t threadState) items() []api.Commit {
	out := make([]api.Commit, 0, len(t.ancestors)+1+len(t.replies))
	out = append(out, t.ancestors...)
	out = append(out, t.commit)
	return append(out, t.replies...)
}

func (t threadState) main() int { return len(t.ancestors) }

type profileState struct {
	user api.User
	feed feed
}

type frame struct {
	screen  screen
	thread  threadState
	profile profileState
}

type toastKind int

const (
	toastInfo toastKind = iota
	toastOK
	toastErr
)

// Model is the root Bubble Tea model.
type Model struct {
	cfg    config.Config
	client *api.Client
	keys   keyMap
	help   help.Model
	spin   spinner.Model

	width  int
	height int

	screen  screen
	section int
	stack   []frame

	me     api.Me
	unread int

	feeds   [4]feed
	notes   notesState
	thread  threadState
	profile profileState

	login   textinput.Model
	session auth.Session

	composer      textarea.Model
	composing     bool
	composeTarget *api.Commit

	showHelp  bool
	busy      bool
	busyLabel string

	toast     string
	toastKind toastKind
	toastID   int

	confirm       string
	pendingDelete string
}

// New constructs the UI. loggedIn is true when a device key was loaded.
func New(cfg config.Config, client *api.Client, loggedIn bool) Model {
	ti := textinput.New()
	ti.Placeholder = "devnull://auth/callback?code=…"
	ti.Prompt = "› "
	ti.PromptStyle = accentStyle
	ti.Cursor.Style = accentStyle
	ti.Focus()

	ta := textarea.New()
	ta.Placeholder = "What are you shipping? (3–4000 characters)"
	ta.CharLimit = 4000
	ta.ShowLineNumbers = false
	ta.Prompt = "│ "
	ta.SetHeight(8)
	ta.FocusedStyle.Prompt = accentStyle
	ta.FocusedStyle.CursorLine = textStyle
	ta.FocusedStyle.Placeholder = faintStyle
	ta.BlurredStyle.Prompt = railStyle
	ta.Cursor.Style = accentStyle

	h := help.New()
	h.ShortSeparator = "  ·  "
	h.Ellipsis = "…"
	h.Styles.ShortKey = helpKey
	h.Styles.ShortDesc = helpDesc
	h.Styles.ShortSeparator = helpSep
	h.Styles.Ellipsis = helpSep

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = accentStyle

	m := Model{
		cfg:      cfg,
		client:   client,
		keys:     newKeys(),
		help:     h,
		spin:     sp,
		login:    ti,
		composer: ta,
		screen:   screenLogin,
	}
	if loggedIn {
		m.screen = screenSection
		m.busy = true
	}
	return m
}

// Init starts the spinner, the poll loop, and either login or the Following feed.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spin.Tick, m.pollCmd()}
	if m.screen == screenLogin {
		cmds = append(cmds, m.startLogin())
	} else {
		cmds = append(cmds, m.loadMe(false), m.loadSection(secFollowing, false))
	}
	return tea.Batch(cmds...)
}

func (m Model) host() string {
	u, err := url.Parse(m.cfg.URL)
	if err != nil || u.Host == "" {
		return m.cfg.URL
	}
	return u.Host
}

func (m Model) bodyHeight() int {
	return max(1, m.height-6)
}

func (m *Model) flash(text string, kind toastKind) tea.Cmd {
	m.toastID++
	m.toast = text
	m.toastKind = kind
	id := m.toastID
	wait := 4 * time.Second
	if kind == toastErr {
		wait = 7 * time.Second
	}
	return tea.Tick(wait, func(time.Time) tea.Msg { return clearToastMsg{id} })
}

func (m *Model) push() {
	m.stack = append(m.stack, frame{screen: m.screen, thread: m.thread, profile: m.profile})
}

func (m *Model) pop() {
	if len(m.stack) == 0 {
		m.screen = screenSection
		return
	}
	f := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	m.screen, m.thread, m.profile = f.screen, f.thread, f.profile
}

// refs returns the length, cursor and scroll offset of whatever list is on screen.
func (m *Model) refs() (int, *int, *int) {
	switch m.screen {
	case screenSection:
		if m.section == secNotes {
			return len(m.notes.items), &m.notes.cursor, &m.notes.offset
		}
		f := &m.feeds[m.section]
		return len(f.list()), &f.cursor, &f.offset
	case screenThread:
		return len(m.thread.items()), &m.thread.cursor, &m.thread.offset
	case screenProfile:
		return len(m.profile.feed.items), &m.profile.feed.cursor, &m.profile.feed.offset
	}
	return 0, nil, nil
}

func (m Model) selected() (api.Commit, bool) {
	switch m.screen {
	case screenSection:
		if m.section == secNotes {
			if m.notes.cursor < len(m.notes.items) {
				if c := m.notes.items[m.notes.cursor].Commit; c != nil && c.Author != nil {
					return *c, true
				}
			}
			return api.Commit{}, false
		}
		list := m.feeds[m.section].list()
		if c := m.feeds[m.section].cursor; c < len(list) && list[c].Author != nil {
			return list[c], true
		}
	case screenThread:
		items := m.thread.items()
		if c := m.thread.cursor; c < len(items) && items[c].Author != nil {
			return items[c], true
		}
	case screenProfile:
		items := m.profile.feed.items
		if c := m.profile.feed.cursor; c < len(items) && items[c].Author != nil {
			return items[c], true
		}
	}
	return api.Commit{}, false
}

func (m Model) isMine(c api.Commit) bool {
	return c.Author != nil && m.me.ID != "" && c.Author.ID == m.me.ID
}

func (m *Model) fix() {
	if m.width == 0 {
		return
	}
	_, cur, off := m.refs()
	if cur == nil {
		return
	}
	lv := m.buildList()
	if len(lv.cards) == 0 {
		*cur, *off = 0, 0
		return
	}
	if *cur >= len(lv.cards) {
		*cur = len(lv.cards) - 1
	}
	heights := make([]int, len(lv.cards))
	for i, c := range lv.cards {
		heights[i] = lipglossHeight(c)
	}
	*off = scroll(heights, *cur, *off, lv.avail)
}

func scroll(heights []int, cursor, offset, avail int) int {
	if cursor < offset {
		return cursor
	}
	for offset < cursor {
		total := 0
		for i := offset; i <= cursor; i++ {
			total += heights[i]
		}
		if total <= avail {
			break
		}
		offset++
	}
	return offset
}

func updateCommits(list []api.Commit, c api.Commit, action string) {
	for i := range list {
		if list[i].ID == c.ID {
			mergeCommit(&list[i], c, action)
		}
	}
}

func mergeCommit(dst *api.Commit, src api.Commit, action string) {
	switch action {
	case "star":
		dst.Starred = src.Starred
		dst.StarsCount = src.StarsCount
	case "save":
		dst.IsSaved = src.IsSaved
	case "status":
		dst.Status = src.Status
	}
}

// pendingIDs lists the user's own commits still awaiting review anywhere on screen.
func (m Model) pendingIDs() []string {
	seen := map[string]bool{}
	var ids []string
	add := func(list ...api.Commit) {
		for _, c := range list {
			if c.Status == "pending" && m.isMine(c) && !seen[c.ID] {
				seen[c.ID] = true
				ids = append(ids, c.ID)
			}
		}
	}
	addState := func(t threadState, p profileState) {
		add(t.commit)
		add(t.ancestors...)
		add(t.replies...)
		add(p.feed.items...)
	}
	for _, f := range m.feeds {
		add(f.items...)
	}
	addState(m.thread, m.profile)
	for _, f := range m.stack {
		addState(f.thread, f.profile)
	}
	return ids
}

func (m *Model) applyCommit(c api.Commit, action string) {
	apply := func(t *threadState, p *profileState) {
		updateCommits(t.ancestors, c, action)
		updateCommits(t.replies, c, action)
		if t.commit.ID == c.ID {
			mergeCommit(&t.commit, c, action)
		}
		updateCommits(p.feed.items, c, action)
	}
	for i := range m.feeds {
		updateCommits(m.feeds[i].items, c, action)
		updateCommits(m.feeds[i].suggestions, c, action)
	}
	apply(&m.thread, &m.profile)
	for i := range m.stack {
		apply(&m.stack[i].thread, &m.stack[i].profile)
	}
	for i := range m.notes.items {
		if n := m.notes.items[i].Commit; n != nil && n.ID == c.ID {
			mergeCommit(n, c, action)
		}
	}
	if action == "save" {
		if c.IsSaved {
			m.feeds[secSaved].loaded = false
		} else {
			m.feeds[secSaved].items = without(m.feeds[secSaved].items, c.ID)
		}
	}
}

func without(list []api.Commit, id string) []api.Commit {
	out := list[:0:0]
	for _, c := range list {
		if c.ID != id {
			out = append(out, c)
		}
	}
	return out
}

func (m *Model) removeCommit(id string) {
	for i := range m.feeds {
		m.feeds[i].items = without(m.feeds[i].items, id)
		m.feeds[i].suggestions = without(m.feeds[i].suggestions, id)
	}
	m.profile.feed.items = without(m.profile.feed.items, id)
	m.thread.replies = without(m.thread.replies, id)
	for i := range m.stack {
		m.stack[i].profile.feed.items = without(m.stack[i].profile.feed.items, id)
		m.stack[i].thread.replies = without(m.stack[i].thread.replies, id)
	}
}
