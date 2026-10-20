package api

import "testing"

func TestSanitizeStringStripsEscapes(t *testing.T) {
	in := "hello\x1b[31mred\x1b]52;c;evil\x07world"
	got := SanitizeString(in)
	if got != "hello[31mred]52;c;evilworld" && got != "hello[31mred52;c;evilworld" {
		// ESC and BEL stripped; the printable leftovers remain.
		if containsControl(got) {
			t.Fatalf("still has controls: %q", got)
		}
	}
	if containsControl(got) {
		t.Fatalf("controls survived: %q", got)
	}
}

func TestSanitizeKeepsText(t *testing.T) {
	in := "Hello @tom 🎉 café\nnext\tline"
	if got := SanitizeString(in); got != in {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeStripsBidi(t *testing.T) {
	in := "abc\u202edef\u202cghi"
	got := SanitizeString(in)
	if got != "abcdefghi" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeWalkCommit(t *testing.T) {
	body := "hi\x1b[0m"
	c := Commit{Body: body, Author: &User{Username: "x\x07y", Name: "n"}}
	sanitizeWalk(&c)
	if containsControl(c.Body) || containsControl(c.Author.Username) {
		t.Fatalf("%+v", c)
	}
}

func containsControl(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != '\n' && r != '\t' || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return true
		}
		if isBidiOverride(r) {
			return true
		}
	}
	return false
}
