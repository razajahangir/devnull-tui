package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/razajahangir/devnull-tui/internal/api"
)

type listView struct {
	cards       []string
	top, bottom string
	empty       string
	avail       int
}

// View renders the current screen.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.width < 50 || m.height < 14 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			mutedStyle.Render("Make this window a little bigger (at least 50×14)."))
	}
	h := m.bodyHeight()
	body := lipgloss.NewStyle().Width(m.width).Height(h).MaxHeight(h).Render(m.viewBody())
	return lipgloss.JoinVertical(lipgloss.Left, m.viewHeader(), body, m.viewFooter())
}

func (m Model) rule() string {
	return railStyle.Render(strings.Repeat("─", m.width))
}

// Header

func (m Model) viewHeader() string {
	brand := logo()
	host := faintStyle.Render(m.host())
	if !m.cfg.IsProduction() {
		host = warnStyle.Bold(true).Render("TEST INSTANCE") + faintStyle.Render(" · ") + host
	}
	right := host + " "
	if m.me.Username != "" {
		right = laneStyle(&m.me.User).Bold(true).Render("@"+m.me.Username) + faintStyle.Render(" · ") + right
	}
	if m.screen == screenLogin {
		line := spread(brand, right, m.width)
		return lipgloss.JoinVertical(lipgloss.Left, line, " "+titleStyle.Render("Sign in"), laneStrip(m.width))
	}
	tabs := m.tabs(false)
	if lipgloss.Width(brand)+lipgloss.Width(tabs)+lipgloss.Width(right)+2 > m.width {
		tabs = m.tabs(true)
	}
	line := spread(brand+" "+tabs, right, m.width)
	return lipgloss.JoinVertical(lipgloss.Left, line, " "+m.crumbs(), laneStrip(m.width))
}

func (m Model) tabs(short bool) string {
	var b strings.Builder
	for i, s := range sections {
		label := s.label
		if short {
			label = s.short
		}
		if i == m.section {
			b.WriteString(tabOn.Render(s.key + " " + label))
		} else {
			b.WriteString(" " + tabKey.Render(s.key) + " " + mutedStyle.Render(label) + " ")
		}
		if i == secNotes && m.unread > 0 {
			b.WriteString(badgeStyle.Render(fmt.Sprint(m.unread)))
		}
	}
	return b.String()
}

func (m Model) crumbs() string {
	sec := sections[m.section]
	if m.screen == screenSection {
		return titleStyle.Render(sec.label) + mutedStyle.Render(" · "+sec.blurb)
	}
	parts := []string{mutedStyle.Render(sec.label)}
	for _, f := range m.stack {
		if f.screen != screenSection {
			parts = append(parts, mutedStyle.Render(frameName(f.screen, f.thread, f.profile)))
		}
	}
	parts = append(parts, titleStyle.Render(frameName(m.screen, m.thread, m.profile)))
	return strings.Join(parts, faintStyle.Render(" › ")) + faintStyle.Render("    esc to go back")
}

func frameName(s screen, t threadState, p profileState) string {
	switch s {
	case screenThread:
		if t.commit.Author != nil {
			return "@" + t.commit.Author.Username + "'s commit " + t.commit.ShortHash
		}
		return "commit"
	case screenProfile:
		if p.user.Username == "" {
			return "profile"
		}
		return "profile @" + p.user.Username
	}
	return ""
}

// Body

func (m Model) viewBody() string {
	switch {
	case m.screen == screenLogin:
		return m.viewLogin()
	case m.composing:
		return m.viewCompose()
	case m.showHelp:
		return m.viewHelp()
	}
	lv := m.buildList()
	mm := m
	_, cur, off := (&mm).refs()
	var out []string
	if lv.top != "" {
		out = append(out, lv.top)
	}
	if len(lv.cards) == 0 {
		out = append(out, lv.empty)
		return strings.Join(out, "\n")
	}
	start := 0
	if off != nil {
		start = min(*off, len(lv.cards)-1)
	}
	_ = cur
	used, last := 0, start
	for i := start; i < len(lv.cards); i++ {
		h := lipgloss.Height(lv.cards[i])
		if used+h > lv.avail && used > 0 {
			break
		}
		out = append(out, lv.cards[i])
		used += h
		last = i
	}
	if lv.bottom != "" && last == len(lv.cards)-1 {
		out = append(out, lv.bottom)
	}
	return strings.Join(out, "\n")
}

func (m Model) buildList() listView {
	w := m.width
	lv := listView{}
	switch m.screen {
	case screenSection:
		if m.section == secNotes {
			lv = m.notesList(w)
		} else {
			lv = m.feedList(w)
		}
	case screenThread:
		lv = m.threadList(w)
	case screenProfile:
		lv = m.profileList(w)
	}
	lv.avail = m.bodyHeight()
	if lv.top != "" {
		lv.avail -= lipgloss.Height(lv.top)
	}
	if lv.bottom != "" {
		lv.avail -= lipgloss.Height(lv.bottom)
	}
	lv.avail = max(3, lv.avail)
	return lv
}

func (m Model) loading(what string) string {
	return "\n  " + m.spin.View() + " " + mutedStyle.Render("Loading "+what+"…")
}

func endMarker(label string) string {
	return faintStyle.Render("  · end of " + label + " ·")
}

func (m Model) feedList(w int) listView {
	sec := sections[m.section]
	f := m.feeds[m.section]
	if !f.loaded {
		return listView{empty: m.loading(sec.label)}
	}
	lv := listView{}
	list := f.list()
	for i, c := range list {
		lv.cards = append(lv.cards, m.commitCard(c, i == f.cursor, w, 4, ""))
	}
	switch {
	case len(f.items) == 0 && len(f.suggestions) > 0:
		lv.top = guide(w,
			"You're not following anyone yet",
			"Here are a few commits to get you started. Open one with enter,\npress p to see its author, then f to follow them.")
	case f.newCount > 0:
		lv.top = "  " + pillOn.Render(fmt.Sprintf("↑ %d new %s", f.newCount, plural(f.newCount, "commit", "commits"))) +
			mutedStyle.Render("  press R to load") + "\n"
	}
	switch {
	case f.loadingMore:
		lv.bottom = "  " + m.spin.View() + mutedStyle.Render(" Loading more…")
	case f.next == nil && len(list) > 0:
		lv.bottom = endMarker(sec.label)
	}
	switch m.section {
	case secFollowing:
		lv.empty = guide(w, "Your Following log is empty",
			"Following shows commits from people you follow. To fill it:\n\n"+
				keyRow("2", "browse Latest to find people")+"\n"+
				keyRow("enter", "open a commit")+"\n"+
				keyRow("p", "jump to its author's profile")+"\n"+
				keyRow("f", "follow them")+"\n\n"+
				"Or press c to write your first commit.")
	case secSaved:
		lv.empty = guide(w, "Nothing saved yet",
			"Press b on any commit to keep it here.\nOnly you can see your saved commits.")
	default:
		lv.empty = guide(w, "No commits yet", "Press c to write the first one.")
	}
	return lv
}

func (m Model) notesList(w int) listView {
	n := m.notes
	if !n.loaded {
		return listView{empty: m.loading("notifications")}
	}
	lv := listView{empty: guide(w, "You're all caught up",
		"Replies, stars, mentions and new followers show up here.\nOpening this list marks everything as read.")}
	for i, item := range n.items {
		isNew := item.ReadAt == nil || (n.readAt != "" && *item.ReadAt >= n.readAt)
		lv.cards = append(lv.cards, m.noteCard(item, i == n.cursor, isNew, w))
	}
	switch {
	case n.loadingMore:
		lv.bottom = "  " + m.spin.View() + mutedStyle.Render(" Loading more…")
	case n.next == nil && len(n.items) > 0:
		lv.bottom = endMarker("notifications")
	}
	return lv
}

func (m Model) threadList(w int) listView {
	t := m.thread
	lv := listView{}
	for i, c := range t.items() {
		sel := i == t.cursor
		switch {
		case i < t.main():
			lv.cards = append(lv.cards, m.commitCard(c, sel, w, 2, " "+laneStyle(c.Author).Render("│")))
		case i == t.main():
			lv.cards = append(lv.cards, m.mainCard(c, sel, w))
		default:
			lv.cards = append(lv.cards, m.commitCard(c, sel, w, 6, ""))
		}
	}
	if t.loadingMore {
		lv.bottom = "  " + m.spin.View() + mutedStyle.Render(" Loading more replies…")
	}
	return lv
}

func (m Model) profileList(w int) listView {
	p := m.profile
	lv := listView{top: m.profileHeader(p.user, w)}
	lv.empty = faintStyle.Render("  @" + p.user.Username + " hasn't posted any commits yet.")
	if p.user.ID == m.me.ID {
		lv.empty = faintStyle.Render("  You haven't posted any commits yet.")
	}
	for i, c := range p.feed.items {
		lv.cards = append(lv.cards, m.commitCard(c, i == p.feed.cursor, w, 4, ""))
	}
	switch {
	case p.feed.loadingMore:
		lv.bottom = "  " + m.spin.View() + mutedStyle.Render(" Loading more…")
	case p.feed.next == nil && len(p.feed.items) > 0:
		lv.bottom = endMarker("@" + p.user.Username + "'s commits")
	}
	return lv
}

// Cards

func imageNote(images []api.CommitImage) []string {
	n := len(images)
	if n == 0 {
		return nil
	}
	return []string{faintStyle.Render(fmt.Sprintf("🖼  %d %s · o to open in the browser", n, plural(n, "image", "images")))}
}

func selBar(selected bool) string {
	if selected {
		return accentStyle.Render("▌ ")
	}
	return "  "
}

// graphGutter prefixes card lines with a git-graph node and line in the
// author's lane color, plus the selection bar.
func graphGutter(lines []string, selected bool, u *api.User) {
	sel := " "
	if selected {
		sel = accentStyle.Render("▌")
	}
	lane := laneStyle(u)
	for i := range lines {
		glyph := "│"
		if i == 0 {
			glyph = "●"
		}
		lines[i] = sel + lane.Render(glyph) + " " + lines[i]
	}
}

func (m Model) stats(c api.Commit, long bool) string {
	stars := fmt.Sprintf("☆ %d", c.StarsCount)
	replies := fmt.Sprintf("↩ %d", c.RepliesCount)
	if long {
		stars += " " + plural(c.StarsCount, "star", "stars")
		replies += " " + plural(c.RepliesCount, "reply", "replies")
	}
	s := mutedStyle.Render(stars)
	if c.Starred {
		s = starStyle.Render("★" + strings.TrimPrefix(stars, "☆"))
	}
	s += "   " + mutedStyle.Render(replies)
	if c.IsSaved {
		saved := "◆"
		if long {
			saved = "◆ saved"
		}
		s += "   " + accentStyle.Render(saved)
	}
	return s
}

func (m Model) commitCard(c api.Commit, selected bool, width, maxLines int, gap string) string {
	inner := max(10, width-5)
	var lines []string
	if c.Author == nil {
		lines = append(lines, faintStyle.Render("[deleted commit]"))
	} else {
		name := laneStyle(c.Author).Bold(true).Render("@" + c.Author.Username)
		meta := name + faintStyle.Render(" · "+ago(c.CreatedAt)+" · ") + mutedStyle.Render(c.ShortHash)
		if c.Status == "pending" {
			meta += warnStyle.Render(" · pending review")
		}
		if c.ParentID != nil && m.screen != screenThread {
			meta += faintStyle.Render(" · reply")
		}
		lines = append(lines, meta)
		body := wrap(c.Body, inner)
		truncated := false
		if maxLines > 0 && len(body) > maxLines {
			body = body[:maxLines]
			body[maxLines-1] = clip(body[maxLines-1], inner-8)
			truncated = true
		}
		for i, l := range body {
			line := richLine(l)
			if truncated && i == len(body)-1 {
				line += faintStyle.Render(" …more")
			}
			lines = append(lines, line)
		}
		if q := c.QuotedCommit; q != nil && q.Author != nil {
			handle := "@" + q.Author.Username
			lines = append(lines, faintStyle.Render("╰ quoting ")+laneStyle(q.Author).Render(handle)+
				faintStyle.Render(clip(": "+oneLine(q.Body), inner-10-len(handle))))
		}
		lines = append(lines, imageNote(c.Images)...)
		lines = append(lines, m.stats(c, false))
	}
	graphGutter(lines, selected, c.Author)
	lines = append(lines, gap)
	return strings.Join(lines, "\n")
}

func (m Model) mainCard(c api.Commit, selected bool, width int) string {
	inner := max(10, width-5)
	var lines []string
	if c.Author == nil {
		lines = append(lines, faintStyle.Render("[deleted commit]"))
	} else {
		head := laneStyle(c.Author).Bold(true).Render("@" + c.Author.Username)
		if c.Author.Name != "" && c.Author.Name != c.Author.Username {
			head += "  " + mutedStyle.Render(c.Author.Name)
		}
		sub := faintStyle.Render(fullTime(c.CreatedAt) + " · " + c.ShortHash)
		if c.Status == "pending" {
			sub += warnStyle.Render(" · pending review, only you can see it")
		}
		lines = append(lines, head, sub, "")
		for _, l := range wrap(c.Body, inner) {
			lines = append(lines, richLine(l))
		}
		if q := c.QuotedCommit; q != nil && q.Author != nil {
			lines = append(lines, "", faintStyle.Render("╭ quoting ")+laneStyle(q.Author).Render("@"+q.Author.Username))
			quoted := wrap(q.Body, inner-2)
			if len(quoted) > 3 {
				quoted = append(quoted[:3], "…")
			}
			for _, l := range quoted {
				lines = append(lines, faintStyle.Render("│ "+l))
			}
		}
		if n := len(c.Images); n > 0 {
			lines = append(lines, "")
			lines = append(lines, imageNote(c.Images)...)
		}
		lines = append(lines, "", m.stats(c, true))
	}
	graphGutter(lines, selected, c.Author)
	var divider string
	switch {
	case !m.thread.repliesLoaded:
		divider = "  " + m.spin.View() + mutedStyle.Render(" Loading replies…")
	case len(m.thread.replies) == 0:
		divider = faintStyle.Render("  ── No replies yet · press r to reply ──")
	default:
		n := max(c.RepliesCount, len(m.thread.replies))
		divider = faintStyle.Render(fmt.Sprintf("  ── %d %s ──", n, plural(n, "reply", "replies")))
	}
	lines = append(lines, "", divider, "")
	return strings.Join(lines, "\n")
}

func (m Model) noteCard(n api.Notification, selected, isNew bool, width int) string {
	bar := selBar(selected)
	inner := max(10, width-6)
	dot := "  "
	if isNew {
		dot = accentStyle.Render("● ")
	}
	actor, rest := noteText(n)
	head := dot
	if actor != "" {
		head += laneStyle(n.Actor).Bold(true).Render(actor)
	}
	head += textStyle.Render(rest) + faintStyle.Render(" · "+ago(n.CreatedAt))
	lines := []string{bar + head}
	if n.Commit != nil && n.Commit.Body != "" {
		lines = append(lines, bar+"  "+faintStyle.Render("“"+clip(oneLine(n.Commit.Body), inner-2)+"”"))
	}
	if n.Type == "rejected" && n.Reason != nil && *n.Reason != "" {
		lines = append(lines, bar+"  "+warnStyle.Render("Reason: "+clip(*n.Reason, inner-8)))
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m Model) profileHeader(u api.User, width int) string {
	inner := max(10, width-4)
	mine := u.ID != "" && u.ID == m.me.ID

	var badges []string
	if mine {
		badges = append(badges, pillOn.Render("you"))
	}
	if u.FollowsYou != nil && *u.FollowsYou && !mine {
		badges = append(badges, pillStyle.Render("follows you"))
	}
	if u.IsMuting != nil && *u.IsMuting {
		badges = append(badges, warnStyle.Render("muted"))
	}
	if u.IsBlocking != nil && *u.IsBlocking {
		badges = append(badges, errStyle.Render("blocked"))
	}

	kicker := mutedStyle.Render("PROFILE")
	if mine {
		kicker = mutedStyle.Render("YOUR PROFILE")
	}
	if len(badges) > 0 {
		kicker += "  " + strings.Join(badges, " ")
	}

	display := u.Name
	if display == "" || display == u.Username {
		display = "@" + u.Username
	}
	ident := []string{
		kicker,
		titleStyle.Render(display),
		laneStyle(&u).Bold(true).Render("@" + u.Username),
	}

	glyph := "👤"
	if r := []rune(strings.TrimSpace(u.Name)); len(r) > 0 {
		glyph = string(r[0])
	} else if r := []rune(u.Username); len(r) > 0 {
		glyph = strings.ToUpper(string(r[0]))
	}
	avatar := laneStyle(&u).Bold(true).Render(" " + glyph + " ")
	identBlock := lipgloss.JoinHorizontal(lipgloss.Top, avatar, "  ", strings.Join(ident, "\n"))

	lines := []string{identBlock, ""}
	if u.Bio != "" {
		bio := wrap(u.Bio, inner)
		if len(bio) > 8 {
			bio = append(bio[:8], "…")
		}
		for _, l := range bio {
			lines = append(lines, textStyle.Render(l))
		}
		lines = append(lines, "")
	}

	row := func(label, value string) {
		if value == "" {
			return
		}
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%-10s", label))+textStyle.Render(clip(value, inner-10)))
	}
	row("company", u.Company)
	row("location", u.Location)
	row("website", displayURL(u.Website))
	row("github", displayURL(u.GitHubURL))
	row("joined", joined(u.CreatedAt))

	count := func(n int, label string) string {
		return titleStyle.Render(fmt.Sprint(n)) + mutedStyle.Render(" "+label)
	}
	stats := count(u.CommitsCount, plural(u.CommitsCount, "commit", "commits")) + "    " +
		count(u.FollowersCount, plural(u.FollowersCount, "follower", "followers")) + "    " +
		count(u.FollowingCount, "following")
	lines = append(lines, "", stats)

	if mine {
		if m.me.PasteisRemaining > 0 || m.me.PasteisReceived > 0 {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d pasteis left today", m.me.PasteisRemaining)))
		}
		lines = append(lines, "", faintStyle.Render("This is you.  o opens your profile on the web"))
	} else if u.IsFollowed != nil && *u.IsFollowed {
		lines = append(lines, "", pillOn.Render("✓ Following")+faintStyle.Render("  f to unfollow · o web profile"))
	} else {
		lines = append(lines, "", pillStyle.Render("+ Follow")+faintStyle.Render("  f to follow · o web profile"))
	}

	lines = append(lines, profileExtras(u, inner)...)

	whose := "@" + u.Username + "'s commits"
	if mine {
		whose = "Your commits"
	}
	lines = append(lines, "", faintStyle.Render("── "+whose+" ──"), "")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func keyRow(k, desc string) string {
	return "  " + helpKey.Render(fmt.Sprintf("%-7s", k)) + "  " + helpDesc.Render(desc)
}

func guide(width int, title, body string) string {
	box := boxStyle.Width(min(72, max(30, width-6))).Render(titleStyle.Render(title) + "\n\n" + mutedStyle.Render(body))
	return lipgloss.NewStyle().MarginLeft(2).MarginTop(1).Render(box)
}

// Overlays

func (m Model) viewLogin() string {
	step := func(n, s string) string { return accentStyle.Render(n) + "  " + textStyle.Render(s) }
	status := m.spin.View() + " " + mutedStyle.Render("Waiting for you to approve in the browser…")
	if m.busy && m.busyLabel != "" {
		status = m.spin.View() + " " + mutedStyle.Render(m.busyLabel)
	}
	content := strings.Join([]string{
		titleStyle.Render("Connect this terminal to /dev/null"),
		mutedStyle.Render("Server: ") + accentStyle.Render(m.cfg.URL),
		"",
		step("1", "Your browser opened the approval page. Sign in with GitHub if asked."),
		step("2", "Click Approve. This terminal becomes one of your devices."),
		step("3", "The browser hands the code back here automatically."),
		"",
		status,
		"",
		faintStyle.Render("Browser says “No Apps Available”? Cancel it, copy the devnull://…"),
		faintStyle.Render("address from the address bar, and paste it here:"),
		m.login.View(),
		"",
		faintStyle.Render("No password or token is stored. Your device key lives in ~/.config/devnull."),
	}, "\n")
	box := boxStyle.Width(min(80, m.width-4)).Render(content)
	return lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Center, box)
}

func (m Model) viewCompose() string {
	title := "New commit"
	if t := m.composeTarget; t != nil && t.Author != nil {
		title = "Reply to @" + t.Author.Username
	}
	lines := []string{titleStyle.Render(title) + faintStyle.Render("  · posting as @"+m.me.Username), ""}
	if t := m.composeTarget; t != nil {
		quoted := wrap(t.Body, min(96, m.width-10))
		if len(quoted) > 3 {
			quoted = append(quoted[:3], "…")
		}
		for _, l := range quoted {
			lines = append(lines, faintStyle.Render("╎ "+l))
		}
		lines = append(lines, "")
	}
	lines = append(lines, m.composer.View(), "")
	n := utf8.RuneCountInString(strings.TrimSpace(m.composer.Value()))
	counter := mutedStyle.Render(fmt.Sprintf("%d / 4000", n))
	switch {
	case n < 3:
		counter = warnStyle.Render(fmt.Sprintf("%d / 4000 · at least 3 characters", n))
	case n > 4000:
		counter = errStyle.Render(fmt.Sprintf("%d / 4000 · too long", n))
	}
	if m.busy {
		counter = m.spin.View() + " " + mutedStyle.Render(m.busyLabel)
	}
	lines = append(lines, counter, "",
		faintStyle.Render("ctrl+s send · esc cancel · enter adds a new line"),
		faintStyle.Render("`code`, ```blocks```, @mentions and links work like on the web."))
	return lipgloss.NewStyle().Padding(1, 0, 0, 2).Render(strings.Join(lines, "\n"))
}

func (m Model) viewHelp() string {
	group := func(title string, rows ...[2]string) string {
		out := []string{titleStyle.Render(title)}
		for _, r := range rows {
			out = append(out, helpKey.Render(fmt.Sprintf("%-11s", r[0]))+" "+helpDesc.Render(r[1]))
		}
		return strings.Join(out, "\n")
	}
	left := strings.Join([]string{
		group("Move around",
			[2]string{"↑ ↓  j k", "move up / down"},
			[2]string{"enter  →", "open"},
			[2]string{"esc  ←", "go back"},
			[2]string{"g  G", "jump to top / bottom"},
			[2]string{"pgup pgdn", "scroll a page"},
			[2]string{"mouse", "wheel scrolls"}),
		group("Sections",
			[2]string{"1", "Following · people you follow"},
			[2]string{"2", "Latest · everyone, newest first"},
			[2]string{"3", "Trending · most engagement"},
			[2]string{"4  S", "Saved · your bookmarks"},
			[2]string{"5  n", "Notifications (marks read)"},
			[2]string{"tab", "next / shift+tab previous"}),
	}, "\n\n")
	right := strings.Join([]string{
		group("On the selected commit",
			[2]string{"c", "write a new commit"},
			[2]string{"r", "reply"},
			[2]string{"s", "star / unstar"},
			[2]string{"b", "save / unsave"},
			[2]string{"p", "open the author's profile"},
			[2]string{"o", "open this commit on the website"},
			[2]string{"d", "delete (your own only)"}),
		group("Profiles",
			[2]string{"f", "follow / unfollow"},
			[2]string{"u", "your own profile"}),
		group("App",
			[2]string{"R", "refresh"},
			[2]string{"?", "show / hide this help"},
			[2]string{"ctrl+l", "sign out"},
			[2]string{"q", "back, or quit at the top"}),
	}, "\n\n")
	cols := lipgloss.JoinHorizontal(lipgloss.Top, left, "      ", right)
	if m.width < 96 {
		cols = left + "\n\n" + right
	}
	note := faintStyle.Render("Not in the terminal yet: messages, search, compose-with-images and polls. Use the website for those.")
	content := brandStyle.Render("Keyboard guide") + faintStyle.Render("   ? or esc to close") + "\n\n" + cols + "\n\n" + note
	box := boxStyle.Render(content)
	if lipgloss.Height(box) > m.bodyHeight() {
		box = boxStyle.Padding(0, 2).Render(strings.ReplaceAll(content, "\n\n", "\n"))
	}
	return lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Top, box)
}

// Footer

func (m Model) viewFooter() string {
	var left string
	switch {
	case m.toast != "":
		switch m.toastKind {
		case toastErr:
			left = errStyle.Render("✗ " + m.toast)
		case toastOK:
			left = okStyle.Render("✓ " + m.toast)
		default:
			left = textStyle.Render("› " + m.toast)
		}
	case m.busy && m.screen != screenLogin && !m.composing:
		label := m.busyLabel
		if label == "" {
			label = "Loading…"
		}
		left = m.spin.View() + " " + mutedStyle.Render(label)
	}
	right := ""
	if !m.composing && !m.showHelp && m.screen != screenLogin {
		mm := m
		if n, cur, _ := (&mm).refs(); cur != nil && n > 0 {
			right = faintStyle.Render(fmt.Sprintf("%d of %d", *cur+1, n))
		}
	}
	if max := m.width - lipgloss.Width(right) - 3; lipgloss.Width(left) > max {
		left = clip(stripToPlain(left), max)
	}
	status := " " + spread(left, right, m.width-2)

	pinned := ""
	if m.screen != screenLogin && !m.composing && !m.showHelp {
		pinned = helpKey.Render("?") + " " + boldAccent.Render("all keys") + " "
	}
	hm := m.help
	hm.Width = m.width - lipgloss.Width(pinned) - 3
	keys := " " + spread(hm.ShortHelpView(m.shortHelp()), pinned, m.width-1)
	return lipgloss.JoinVertical(lipgloss.Left, m.rule(), status, keys)
}

func stripToPlain(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			esc = true
		case esc && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'):
			esc = false
		case !esc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (m Model) shortHelp() []key.Binding {
	k := m.keys
	mv := key.NewBinding(key.WithKeys("j"), key.WithHelp("↑↓", "move"))
	sections := withHelp(k.NextTab, "tab/1-5", "sections")
	switch {
	case m.screen == screenLogin:
		return []key.Binding{k.Reopen, k.Submit, withHelp(k.Quit, "esc", "quit")}
	case m.composing:
		return []key.Binding{k.Send, k.Cancel}
	case m.showHelp:
		return []key.Binding{withHelp(k.Help, "esc/?", "close help")}
	}
	c, has := m.selected()
	star := withHelp(k.Star, "s", "star")
	if c.Starred {
		star = withHelp(k.Star, "s", "unstar")
	}
	save := withHelp(k.Save, "b", "save")
	if c.IsSaved {
		save = withHelp(k.Save, "b", "unsave")
	}
	var b []key.Binding
	switch m.screen {
	case screenSection:
		b = append(b, mv)
		if m.section == secNotes {
			if len(m.notes.items) > 0 {
				b = append(b, k.Open)
			}
			return append(b, sections, k.Refresh, k.Quit)
		}
		if has {
			b = append(b, k.Open, k.Reply, star, save, k.Profile, k.Browser)
		}
		return append(b, k.Compose, sections, k.Quit)
	case screenThread:
		b = append(b, mv)
		if m.thread.cursor != m.thread.main() {
			b = append(b, k.Open)
		}
		if has {
			b = append(b, k.Reply, star, save, k.Profile)
			if m.isMine(c) {
				b = append(b, k.Delete)
			}
		}
		return append(b, k.Back, k.Browser)
	case screenProfile:
		b = append(b, mv)
		if m.profile.user.ID != m.me.ID {
			label := "follow"
			if u := m.profile.user; u.IsFollowed != nil && *u.IsFollowed {
				label = "unfollow"
			}
			b = append(b, withHelp(k.Follow, "f", label))
		}
		if has {
			b = append(b, k.Open, star, k.Reply)
		}
		return append(b, k.Back, k.Browser)
	}
	return []key.Binding{k.Quit}
}
