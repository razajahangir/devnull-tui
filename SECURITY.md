# Security

Please **do not** open a public issue for a vulnerability.

Report it privately through [GitHub Security Advisories](https://github.com/razajahangir/devnull-tui/security/advisories/new) on this repository.

Include:

- What you did
- What you expected
- What happened
- Client version (`devnull --version`) and OS

We will acknowledge the report and work on a fix before any public disclosure.

This client stores an Ed25519 **device** key (not your GitHub SSH key) at `~/.config/devnull/instances/<host>/device.ed25519`. If you believe a key was leaked, delete that file, sign in again, and revoke the old device under Settings → Devices on the instance.
