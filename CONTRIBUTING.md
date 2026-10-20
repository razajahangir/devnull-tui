# Contributing

Thanks for taking the time. This client is an unofficial community project.

## Setup

Go 1.22+ and `make`:

```bash
git clone https://github.com/<you>/devnull-tui.git
cd devnull-tui
make build
make test
make check
```

`make check` runs `gofmt`, `go vet`, `go test -race`, and golangci-lint / govulncheck when those tools are installed.

## Running against a local server

```bash
./devnull --url http://localhost:8000
```

That uses `~/.config/devnull/instances/127.0.0.1_8000/device.ed25519`, so your production key is untouched. The header shows **TEST INSTANCE**.

## Style

- `gofmt` is required. CI fails if it would change a file.
- Exported identifiers need a doc comment. Package docs live in `doc.go`.
- Comments explain invariants (signature canonical form, why we sanitize at decode time), not line-by-line narration.
- Do not log or print device keys, PKCE verifiers, or authorization codes.

## Adding an API endpoint

1. Add a method on `api.Client` in `internal/api/client.go`. Put IDs and usernames through `url.PathEscape`.
2. Add types in `internal/api/types.go`. JSON from the server is sanitized automatically after decode.
3. Wire the UI in `internal/ui` (`cmds.go` for the HTTP call, `update.go` for the message, `view.go` for rendering).
4. Cover the client method with an `httptest` case if it has interesting path or error behaviour.

## Adding a setting

1. Add the field to `fileConfig` and `Config` in `internal/config/config.go`.
2. Give it a default, an env var (`DEVNULL_…`), and a flag.
3. Reject unknown JSON keys (already the decoder default).
4. Document it in `config.example.json` and the README table.
5. Add a precedence test.

## UI tests

`internal/ui/view_test.go` drives the model with `tea.KeyMsg` and `tea.WindowSizeMsg`. Keep views from overflowing the window. When you change refresh behaviour, add a test that the right `tea.Cmd` is returned rather than screenshotting ANSI.

## Pull requests

- One concern per PR.
- Include tests for security-sensitive changes (sanitizer, URL allowlist, HTTP client).
- Do not add secrets, API tokens, or production keys.

## Releasing

Maintainers only:

```bash
git tag v0.1.0
git push --tags
```

The release workflow runs tests, then GoReleaser, then publishes build-provenance attestations. Archive names are versionless (`devnull_linux_amd64.tar.gz`) so README download links stay stable.
