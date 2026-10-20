package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up, Down, Top, Bottom, PageUp, PageDown key.Binding
	Open, Back, NextTab, PrevTab            key.Binding
	Compose, Reply, Star, Save, Profile     key.Binding
	Browser, Delete, Follow, Me, Refresh    key.Binding
	Help, Quit, SignOut                     key.Binding
	Send, Cancel, Submit, Reopen            key.Binding
	Sections                                [5]key.Binding
}

func newKeys() keyMap {
	k := keyMap{
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d", " "), key.WithHelp("pgdn/space", "page down")),
		Open:     key.NewBinding(key.WithKeys("enter", "right", "l"), key.WithHelp("enter", "open")),
		Back:     key.NewBinding(key.WithKeys("esc", "backspace", "left", "h"), key.WithHelp("esc", "back")),
		NextTab:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next section")),
		PrevTab:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev section")),
		Compose:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "new commit")),
		Reply:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reply")),
		Star:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "star")),
		Save:     key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "save")),
		Profile:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "author")),
		Browser:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open on the web")),
		Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Follow:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "follow")),
		Me:       key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "my profile")),
		Refresh:  key.NewBinding(key.WithKeys("R", "ctrl+r"), key.WithHelp("R", "refresh")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "all keys")),
		Quit:     key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		SignOut:  key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "sign out")),
		Send:     key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "send")),
		Cancel:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		Submit:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit pasted URL")),
		Reopen:   key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "reopen browser")),
	}
	for i, s := range sections {
		k.Sections[i] = key.NewBinding(key.WithKeys(s.key), key.WithHelp(s.key, s.label))
	}
	k.Sections[3].SetKeys("4", "S")
	k.Sections[4].SetKeys("5", "n")
	return k
}

func withHelp(b key.Binding, k, desc string) key.Binding {
	b.SetHelp(k, desc)
	return b
}
