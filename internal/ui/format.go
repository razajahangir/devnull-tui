package ui

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/razajahangir/devnull-tui/internal/api"
)

func lipglossHeight(s string) int { return lipgloss.Height(s) }

func parseTime(ts string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, ts)
	return t, err == nil
}

func ago(ts string) string {
	t, ok := parseTime(ts)
	if !ok {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case t.Year() == time.Now().Year():
		return t.Local().Format("Jan 2")
	}
	return t.Local().Format("Jan 2, 2006")
}

func joined(ts string) string {
	t, ok := parseTime(ts)
	if !ok {
		return ""
	}
	return t.Local().Format("Jan 2006")
}

func commitPageURL(instance string, c api.Commit) string {
	if strings.TrimSpace(c.HTMLURL) != "" {
		return strings.TrimSpace(c.HTMLURL)
	}
	if c.Author == nil || c.Author.Username == "" || c.ID == "" {
		return ""
	}
	return strings.TrimRight(instance, "/") + "/@" + url.PathEscape(c.Author.Username) + "/commits/" + url.PathEscape(c.ID)
}

func displayURL(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	return strings.TrimRight(s, "/")
}

func fullTime(ts string) string {
	t, ok := parseTime(ts)
	if !ok {
		return ""
	}
	return t.Local().Format("Jan 2, 2006 at 15:04")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func clip(s string, n int) string {
	if n <= 1 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func wrap(s string, width int) []string {
	width = max(10, width)
	s = strings.ReplaceAll(strings.TrimSpace(s), "\t", "    ")
	var out []string
	blank := false
	for _, para := range strings.Split(s, "\n") {
		if strings.TrimSpace(para) == "" {
			if !blank && len(out) > 0 {
				out = append(out, "")
			}
			blank = true
			continue
		}
		blank = false
		for _, l := range strings.Split(lipgloss.NewStyle().Width(width).Render(para), "\n") {
			out = append(out, strings.TrimRight(l, " "))
		}
	}
	return out
}

func spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left + "  " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

func noteText(n api.Notification) (actor, rest string) {
	if n.Actor != nil {
		actor = "@" + n.Actor.Username
	}
	if n.ActorsCount > 1 {
		others := n.ActorsCount - 1
		rest = fmt.Sprintf(" and %d %s", others, plural(others, "other", "others"))
	}
	switch n.Type {
	case "followed":
		rest += " followed you"
	case "replied":
		rest += " replied to your commit"
	case "mentioned":
		rest += " mentioned you"
	case "quoted":
		rest += " quoted your commit"
	case "starred":
		rest += " starred your commit"
	case "approved":
		return "", "Your commit was approved and is now public"
	case "rejected":
		return "", "A moderator rejected your commit"
	case "leveled_up":
		return "", "You leveled up!"
	default:
		rest += " · " + n.Type
	}
	if actor == "" {
		actor = "Someone"
	}
	return actor, rest
}

func profileExtras(u api.User, width int) []string {
	var lines []string
	if a := u.Activity; a != nil && len(a.Buckets) > 0 {
		total := 0
		for _, b := range a.Buckets {
			total += b.Count
		}
		lines = append(lines, "",
			mutedStyle.Render("Commit activity")+faintStyle.Render(fmt.Sprintf("  %d in %d days", total, len(a.Buckets))),
			activityHeat(a.Buckets)+"  "+activityLabels(a.Buckets),
		)
	}
	if s := u.Streaks; s != nil && (s.Longest > 0 || s.Current > 0) {
		lines = append(lines, mutedStyle.Render("longest streak ")+
			titleStyle.Render(fmt.Sprintf("%d days", s.Longest))+
			mutedStyle.Render("  ·  current ")+
			okStyle.Render(fmt.Sprint(s.Current)))
	}
	if ach := u.Achievements; ach != nil && (ach.Level > 0 || ach.Rank != "") {
		rank := ach.Rank
		if rank == "" {
			rank = "user"
		}
		head := accentStyle.Render(fmt.Sprintf("Lv. %d", ach.Level)) + mutedStyle.Render(" · ") +
			titleStyle.Render(rank)
		if ach.XP > 0 {
			head += faintStyle.Render(fmt.Sprintf("  %d XP", ach.XP))
		}
		if n := unlockedCount(ach.Badges); len(ach.Badges) > 0 {
			head += faintStyle.Render(fmt.Sprintf("  ·  %d/%d unlocked", n, len(ach.Badges)))
		}
		lines = append(lines, "", head)
		if ach.NextLevelXP > 0 {
			bar := xpBar(ach.LevelProgress, min(20, max(8, width-28)))
			toNext := ach.NextLevelXP - ach.XP
			if toNext < 0 {
				toNext = 0
			}
			next := nextRank(ach)
			tail := fmt.Sprintf("%d/%d", ach.XP, ach.NextLevelXP)
			if next != "" && toNext > 0 {
				tail += fmt.Sprintf("  %d XP to %s", toNext, next)
			}
			lines = append(lines, bar+"  "+faintStyle.Render(tail))
		}
		if len(ach.Badges) > 0 {
			lines = append(lines, wrapBadges(ach.Badges, width)...)
		}
	}
	return lines
}

func activityHeat(buckets []api.UserActivityBucket) string {
	maxc := 1
	for _, b := range buckets {
		if b.Count > maxc {
			maxc = b.Count
		}
	}
	var b strings.Builder
	for i, bucket := range buckets {
		if i > 0 {
			b.WriteByte(' ')
		}
		frac := float64(bucket.Count) / float64(maxc)
		cell, col := "··", faint
		switch {
		case bucket.Count == 0:
		case frac < 0.34:
			cell, col = "▒▒", ok
		case frac < 0.67:
			cell, col = "▓▓", ok
		default:
			cell, col = "██", ok
		}
		b.WriteString(lipgloss.NewStyle().Foreground(col).Render(cell))
	}
	return b.String()
}

func activityLabels(buckets []api.UserActivityBucket) string {
	parts := make([]string, 0, len(buckets))
	for _, b := range buckets {
		t, err := time.Parse("2006-01-02", b.Date)
		if err != nil {
			parts = append(parts, "  ")
			continue
		}
		parts = append(parts, t.Format("Mon")[:2])
	}
	return faintStyle.Render(strings.Join(parts, " "))
}

func xpBar(progress, width int) string {
	if width < 4 {
		width = 4
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	filled := progress * width / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return okStyle.Render(bar)
}

func nextRank(ach *api.UserAchievements) string {
	if ach == nil {
		return ""
	}
	for _, r := range ach.Ranks {
		if !r.Achieved && r.Rank != "" {
			return r.Rank
		}
	}
	return ""
}

func unlockedCount(badges []api.UserBadge) int {
	n := 0
	for _, b := range badges {
		if b.Unlocked {
			n++
		}
	}
	return n
}

func wrapBadges(badges []api.UserBadge, width int) []string {
	if len(badges) == 0 {
		return nil
	}
	width = max(10, width)
	var lines []string
	var cur string
	for _, b := range badges {
		name := b.Name
		if name == "" {
			name = b.ID
		}
		piece := name
		if b.Unlocked {
			piece = "✓ " + name
		} else if b.Target > 1 && b.Progress > 0 {
			piece = fmt.Sprintf("%s(%d/%d)", name, b.Progress, b.Target)
		}
		styled := faintStyle.Render(piece)
		if b.Unlocked {
			styled = okStyle.Render(piece)
		}
		sep := "  "
		if cur == "" {
			sep = ""
		}
		if lipgloss.Width(cur)+lipgloss.Width(sep)+lipgloss.Width(styled) > width && cur != "" {
			lines = append(lines, cur)
			cur = styled
			continue
		}
		cur += sep + styled
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) > 3 {
		lines = lines[:3]
	}
	return lines
}
