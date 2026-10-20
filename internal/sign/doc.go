// Package sign implements SSHSIG (namespace "devnull-api") for /api/v1.
//
// Canonical is:
//
//	<unix-timestamp>\n<nonce>\n<METHOD>\n<request-uri>\n<body>
//
// The body is the raw JSON bytes (empty for no payload). The host is not
// included, so a signature minted for one instance must never be sent to
// another. Package config keeps a separate device key per host for that
// reason.
package sign
