package sign

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	// Magic is the SSHSIG preamble.
	Magic = "SSHSIG"
	// Namespace is the SSHSIG namespace the /dev/null API expects.
	Namespace = "devnull-api"
)

var (
	// ErrMalformed means the signature blob could not be parsed.
	ErrMalformed = errors.New("malformed SSH signature")
	// ErrVerify means the signature does not match the message.
	ErrVerify = errors.New("invalid SSH signature")
	// ErrKeyType means the signature is not ssh-ed25519.
	ErrKeyType = errors.New("unsupported SSH key type")
	// ErrHash means the hash algorithm is not sha256 or sha512.
	ErrHash = errors.New("unsupported SSH signature hash algorithm")
	// ErrVersion means the SSHSIG version is not 1.
	ErrVersion = errors.New("unsupported SSH signature version")
	// ErrNamespace means the signature was issued for a different namespace.
	ErrNamespace = errors.New("SSH signature namespace mismatch")
)

// Canonical is the exact byte string that is hashed and signed.
func Canonical(timestamp int64, nonce, method, target, body string) string {
	return strings.Join([]string{
		fmt.Sprintf("%d", timestamp),
		nonce,
		strings.ToUpper(method),
		target,
		body,
	}, "\n")
}

// PublicBlob is the SSH wire encoding of an Ed25519 public key.
func PublicBlob(pub ed25519.PublicKey) []byte {
	return concat(packString([]byte("ssh-ed25519")), packString(pub))
}

// OpenSSHPublic is the single-line authorized_keys form of pub.
func OpenSSHPublic(pub ed25519.PublicKey) string {
	return "ssh-ed25519 " + base64.StdEncoding.EncodeToString(PublicBlob(pub))
}

// Fingerprint is the SHA256 OpenSSH fingerprint of pub.
func Fingerprint(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(PublicBlob(pub))
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// Sign returns a base64 SSHSIG over message.
func Sign(priv ed25519.PrivateKey, message string) string {
	hash := sha512.Sum512([]byte(message))
	signedData := concat(
		[]byte(Magic),
		packString([]byte(Namespace)),
		packString(nil),
		packString([]byte("sha512")),
		packString(hash[:]),
	)
	sig := ed25519.Sign(priv, signedData)
	inner := concat(packString([]byte("ssh-ed25519")), packString(sig))
	pub := priv.Public().(ed25519.PublicKey)
	body := concat(
		[]byte(Magic),
		packUint32(1),
		packString(PublicBlob(pub)),
		packString([]byte(Namespace)),
		packString(nil),
		packString([]byte("sha512")),
		packString(inner),
	)
	return base64.StdEncoding.EncodeToString(body)
}

// Verify checks an SSHSIG produced by Sign (or ssh-keygen -Y sign).
func Verify(pub ed25519.PublicKey, message, signature string) error {
	blob, err := decodeSignature(signature)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(blob), Magic) {
		return ErrMalformed
	}
	rest := blob[len(Magic):]
	version, rest, err := unpackUint32(rest)
	if err != nil {
		return ErrMalformed
	}
	if version != 1 {
		return ErrVersion
	}
	pubBlob, rest, err := unpackString(rest)
	if err != nil {
		return ErrMalformed
	}
	namespace, rest, err := unpackString(rest)
	if err != nil {
		return ErrMalformed
	}
	reserved, rest, err := unpackString(rest)
	if err != nil {
		return ErrMalformed
	}
	hashAlg, rest, err := unpackString(rest)
	if err != nil {
		return ErrMalformed
	}
	inner, rest, err := unpackString(rest)
	if err != nil || len(rest) != 0 {
		return ErrMalformed
	}
	if string(namespace) != Namespace {
		return ErrNamespace
	}
	if string(hashAlg) != "sha512" && string(hashAlg) != "sha256" {
		return ErrHash
	}
	if string(pubBlob) != string(PublicBlob(pub)) {
		return ErrVerify
	}
	alg, inner, err := unpackString(inner)
	if err != nil {
		return ErrMalformed
	}
	sig, inner, err := unpackString(inner)
	if err != nil || len(inner) != 0 {
		return ErrMalformed
	}
	if string(alg) != "ssh-ed25519" {
		return ErrKeyType
	}
	var digest []byte
	if string(hashAlg) == "sha512" {
		sum := sha512.Sum512([]byte(message))
		digest = sum[:]
	} else {
		sum := sha256.Sum256([]byte(message))
		digest = sum[:]
	}
	signedData := concat(
		[]byte(Magic),
		packString(namespace),
		packString(reserved),
		packString(hashAlg),
		packString(digest),
	)
	if !ed25519.Verify(pub, signedData, sig) {
		return ErrVerify
	}
	return nil
}

func decodeSignature(signature string) ([]byte, error) {
	trimmed := strings.TrimSpace(signature)
	trimmed = strings.ReplaceAll(trimmed, "-----BEGIN SSH SIGNATURE-----", "")
	trimmed = strings.ReplaceAll(trimmed, "-----END SSH SIGNATURE-----", "")
	trimmed = strings.Join(strings.Fields(trimmed), "")
	blob, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, ErrMalformed
	}
	return blob, nil
}

func packUint32(n uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	return b
}

func packString(s []byte) []byte {
	if uint64(len(s)) > math.MaxUint32 {
		panic("sshsig string longer than 4 GiB")
	}
	return concat(packUint32(uint32(len(s))), s) //nolint:gosec // length checked above
}

func unpackUint32(b []byte) (uint32, []byte, error) {
	if len(b) < 4 {
		return 0, nil, ErrMalformed
	}
	return binary.BigEndian.Uint32(b[:4]), b[4:], nil
}

func unpackString(b []byte) ([]byte, []byte, error) {
	n, rest, err := unpackUint32(b)
	if err != nil || uint64(len(rest)) < uint64(n) {
		return nil, nil, ErrMalformed
	}
	return rest[:n], rest[n:], nil
}

func concat(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
