package api

import (
	"reflect"
	"unicode/utf8"
)

// SanitizeString strips terminal-control and bidirectional-override
// characters from untrusted text. Newlines and tabs are kept. The result
// is always valid UTF-8.
func SanitizeString(s string) string {
	if s == "" {
		return s
	}
	out := make([]rune, 0, utf8.RuneCountInString(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			out = append(out, r)
		case r == '\r':
			// Drop CR so \r\n collapses to \n and bare CR cannot rewind a line.
		case r < 0x20 || r == 0x7f:
			// C0 controls and DEL, including ESC (used by CSI/OSC sequences).
		case r >= 0x80 && r <= 0x9f:
			// C1 controls.
		case isBidiOverride(r):
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

func isBidiOverride(r rune) bool {
	switch r {
	case 0x061c, // Arabic letter mark
		0x200e, 0x200f, // LTR / RTL marks
		0x202a, 0x202b, 0x202c, 0x202d, 0x202e, // embedding / override
		0x2066, 0x2067, 0x2068, 0x2069: // isolate
		return true
	}
	return false
}

// sanitizeWalk rewrites every string in v. v must be a pointer to the
// decoded JSON destination.
func sanitizeWalk(v any) {
	if v == nil {
		return
	}
	sanitizeValue(reflect.ValueOf(v))
}

func sanitizeValue(v reflect.Value) {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		if v.CanSet() {
			v.SetString(SanitizeString(v.String()))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if f.CanSet() {
				sanitizeValue(f)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			sanitizeValue(v.Index(i))
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			sanitizeValue(v.MapIndex(key))
		}
	}
}
