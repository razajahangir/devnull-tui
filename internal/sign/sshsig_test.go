package sign

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
)

func TestCanonicalEmptyBodyEndsWithNewline(t *testing.T) {
	got := Canonical(1791000000, "4f3c2b1a09e8d7c6b5a4938271605f4e", "GET", "/api/v1/log/latest?cursor=abc", "")
	want := "1791000000\n4f3c2b1a09e8d7c6b5a4938271605f4e\nGET\n/api/v1/log/latest?cursor=abc\n"
	if got != want {
		t.Fatalf("canonical mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestSignRoundTrip(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	msg := Canonical(1791000000, "0123456789abcdef0123456789abcdef", "POST", "/api/v1/commits", `{"body":"hello"}`)
	sig := Sign(priv, msg)
	pub := priv.Public().(ed25519.PublicKey)
	if err := Verify(pub, msg, sig); err != nil {
		t.Fatal(err)
	}
	if err := Verify(pub, msg+"x", sig); err == nil {
		t.Fatal("expected verify to fail on tampered message")
	}
}

func TestVerifyGoldenSshKeygen(t *testing.T) {
	line := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIrhcHdLGpoKRYqI6eBWDeFu3VITYyTHDGiQ7QhYXCPa vector"
	fields := strings.Fields(line)
	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		t.Fatal(err)
	}
	typeStr, rest, err := unpackString(blob)
	if err != nil {
		t.Fatal(err)
	}
	pub, rest, err := unpackString(rest)
	if err != nil || len(rest) != 0 || string(typeStr) != "ssh-ed25519" {
		t.Fatal("bad public key blob")
	}
	msg := "1791000000\n4f3c2b1a09e8d7c6b5a4938271605f4e\nGET\n/api/v1/log/latest?cursor=abc\n"
	sig := "U1NIU0lHAAAAAQAAADMAAAALc3NoLWVkMjU1MTkAAAAgiuFwd0samgpFiojp4FYN4W7dUhNjJMcMaJDtCFhcI9oAAAALZGV2bnVsbC1hcGkAAAAAAAAABnNoYTUxMgAAAFMAAAALc3NoLWVkMjU1MTkAAABAKZ8la0GdO1nis8Bj7T2TApm89jTsN0of83/YJILbEsR6k/1q08FOHkSl3eZRIPoRVIjJTkXvqWhKlGMe31QwBw=="
	if err := Verify(ed25519.PublicKey(pub), msg, sig); err != nil {
		t.Fatal(err)
	}
}
