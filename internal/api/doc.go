// Package api is the signed HTTP client for a /dev/null instance.
//
// Every request is an SSHSIG (namespace "devnull-api") over a canonical
// string of timestamp, nonce, method, request URI, and body. The host is
// not part of the signed payload, so callers must keep a separate device
// key per instance.
//
// JSON responses are sanitized before they are returned: control
// characters, terminal escape sequences, and Unicode bidirectional
// overrides are stripped so untrusted commit text cannot drive the
// terminal.
package api
