package ui

import (
	"hash/fnv"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/razajahangir/devnull-tui/internal/api"
)

// Dark theme palette from dev-null.sh.
var (
	primary = lipgloss.Color("#7aa2f7")
	text    = lipgloss.Color("#ececec")
	muted   = lipgloss.Color("#a1a1a6")
	faint   = lipgloss.Color("#6e6e73")
	rail    = lipgloss.Color("#36363a")
	surface = lipgloss.Color("#232326")
	danger  = lipgloss.Color("#ff6b6b")
	brand   = lipgloss.Color("#ff2d20")
	ink     = lipgloss.Color("#111112")

	lanes = []lipgloss.Color{
		"#ff8a4c", // orange
		"#51cf66", // green
		"#da77f2", // purple
		"#3bc9db", // cyan
		"#f06595", // pink
		"#fcc419", // yellow
	}
	ok   = lanes[1]
	warn = lanes[5]
	star = lanes[5]

	brandStyle  = lipgloss.NewStyle().Bold(true).Foreground(text)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(text)
	accentStyle = lipgloss.NewStyle().Foreground(primary)
	boldAccent  = lipgloss.NewStyle().Bold(true).Foreground(primary)
	textStyle   = lipgloss.NewStyle().Foreground(text)
	mutedStyle  = lipgloss.NewStyle().Foreground(muted)
	faintStyle  = lipgloss.NewStyle().Foreground(faint)
	railStyle   = lipgloss.NewStyle().Foreground(rail)
	errStyle    = lipgloss.NewStyle().Foreground(danger)
	warnStyle   = lipgloss.NewStyle().Foreground(warn)
	starStyle   = lipgloss.NewStyle().Foreground(star)
	okStyle     = lipgloss.NewStyle().Foreground(ok)
	linkStyle   = lipgloss.NewStyle().Foreground(primary)

	tabOn  = lipgloss.NewStyle().Bold(true).Foreground(ink).Background(primary).Padding(0, 1)
	tabOff = lipgloss.NewStyle().Foreground(muted).Padding(0, 1)
	tabKey = lipgloss.NewStyle().Foreground(faint)

	badgeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(brand).Padding(0, 1)
	pillStyle  = lipgloss.NewStyle().Foreground(text).Background(surface).Padding(0, 1)
	pillOn     = lipgloss.NewStyle().Bold(true).Foreground(ink).Background(primary).Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(rail).
			Padding(1, 2)

	helpKey  = lipgloss.NewStyle().Bold(true).Foreground(primary)
	helpDesc = lipgloss.NewStyle().Foreground(muted)
	helpSep  = lipgloss.NewStyle().Foreground(faint)
)

// logo renders "/dev/null" with the second slash in lane 0, like the web logo.
func logo() string {
	return " " + brandStyle.Render("/dev") +
		lipgloss.NewStyle().Bold(true).Foreground(lanes[0]).Render("/") +
		brandStyle.Render("null") + " "
}

// laneStrip is the six-color bar under the web's top banner.
func laneStrip(width int) string {
	var b strings.Builder
	for i, c := range lanes {
		n := width*(i+1)/len(lanes) - width*i/len(lanes)
		b.WriteString(lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("▀", n)))
	}
	return b.String()
}

// laneColor resolves a user's lane: an index into lanes, a custom hex color,
// or a stable pick from their ID when unset.
func laneColor(u *api.User) lipgloss.Color {
	if u == nil {
		return muted
	}
	switch v := u.Lane.(type) {
	case float64:
		if i := int(v); i >= 0 && i < len(lanes) {
			return lanes[i]
		}
	case string:
		if readableHex(v) {
			return lipgloss.Color(v)
		}
		if i, err := strconv.Atoi(v); err == nil && i >= 0 && i < len(lanes) {
			return lanes[i]
		}
	}
	h := fnv.New32a()
	h.Write([]byte(u.ID + u.Username))
	return lanes[int(h.Sum32())%len(lanes)]
}

// readableHex rejects custom lanes too dark to see on a dark terminal.
func readableHex(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	n, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return false
	}
	r, g, b := n>>16&0xff, n>>8&0xff, n&0xff
	return r*299+g*587+b*114 >= 60000
}

func laneStyle(u *api.User) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(laneColor(u))
}

// richLine colors @mentions and links in an already-wrapped body line.
func richLine(l string) string {
	var b, plain strings.Builder
	flush := func() {
		if plain.Len() > 0 {
			b.WriteString(textStyle.Render(plain.String()))
			plain.Reset()
		}
	}
	for i, w := range strings.Split(l, " ") {
		if i > 0 {
			plain.WriteByte(' ')
		}
		if len(w) > 1 && w[0] == '@' || strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
			flush()
			b.WriteString(linkStyle.Render(w))
			continue
		}
		plain.WriteString(w)
	}
	flush()
	return b.String()
}
