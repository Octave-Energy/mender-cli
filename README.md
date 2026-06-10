Mender CLI
==========

[![CI](https://github.com/Octave-Energy/mender-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/Octave-Energy/mender-cli/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mendersoftware/mender-cli)](https://goreportcard.com/report/github.com/mendersoftware/mender-cli)

`mender-cli` is a standalone command-line tool for working with the
[Mender server management APIs](https://docs.mender.io/apis/management-apis).
It makes it easy to script and automate common Mender operations — logging in,
uploading and managing artifacts, listing devices and inventory, opening remote
terminals, forwarding ports and copying files — from CI pipelines and other
backend/cloud systems.

- **Command reference:** see [CLI.md](./CLI.md) for per-command flags and examples.
- **Mender docs:** [set up mender-cli](https://docs.mender.io/server-integration/using-the-apis#set-up-mender-cli).

## Contents

- [Installation](#installation)
- [Quick start](#quick-start)
- [Authentication](#authentication)
- [Command overview](#command-overview)
- [Configuration file](#configuration-file)
- [Autocompletion](#autocompletion)
- [Building from source](#building-from-source)
- [Repository layout](#repository-layout)
- [CI/CD and releases](#cicd-and-releases)
- [Contributing](#contributing)
- [License](#license)
- [Security disclosure](#security-disclosure)
- [Connect with us](#connect-with-us)

## Installation

Download a prebuilt binary from the
[Mender Docs downloads page](https://docs.mender.io/downloads#mender-cli), or
from this repository's [GitHub Releases](../../releases) (each release ships
`tar.gz`/`zip` archives for Linux, macOS and Windows on `amd64` and `arm64`,
plus a `SHA256SUMS.txt`).

To build it yourself, see [Building from source](#building-from-source).

## Quick start

```console
# 1. Authenticate (interactive prompts for anything omitted)
mender-cli --server https://hosted.mender.io login --username me@example.com

# 2. Use it
mender-cli artifacts list
mender-cli inventory devices list -f hostname=my-gateway
mender-cli artifacts upload ./release-1.mender
```

The `--server` URL is remembered via the [configuration file](#configuration-file)
or can be passed on every invocation. If the scheme is omitted, `https://` is
assumed.

## Authentication

Every command (except `login`, `token`, `version` and `completion`) needs a
valid credential. There are three ways to provide one.

### 1. Username / password login (with optional 2FA)

`login` authenticates against the server and stores a session token locally, so
subsequent commands work without re-authenticating.

```console
# Fully interactive: prompts for username, then a masked password
mender-cli login

# Provide the username up front, get prompted for the password
mender-cli login --username me@example.com

# Non-interactive
mender-cli login --username me@example.com --password secret

# With two-factor authentication enabled on the account
mender-cli login --username me@example.com --password secret --2fa-code 123456
```

You can also set the username/password/server in the
[configuration file](#configuration-file).

### 2. Personal Access Token (PAT)

Instead of `login`, you can persist a Personal Access Token — generated in the
Mender UI under your user account — so every command authenticates with it. The
token is saved to a platform-specific location; you never need to know exactly
where.

```console
# Store interactively (masked prompt)
mender-cli token set

# Or pipe it in from a secret manager (no shell-history leak)
op read "op://Personal/Mender PAT/credential" | mender-cli token set

# Inspect the stored token (decoded JWT table by default)
mender-cli token show
mender-cli token show --json     # decoded header + payload as JSON
mender-cli token show --raw      # the verbatim token, e.g. for Authorization headers

# Where is it stored, and how do I remove it?
mender-cli token path
mender-cli token clear
```

After `token set`, subsequent commands (e.g. `mender-cli artifacts list`) use
the saved token automatically — no extra flags required.

### 3. Per-invocation token flags

For one-off or CI usage you can pass a token directly without storing it:

```console
mender-cli --token-value "$MENDER_JWT" artifacts list   # token value (API key)
mender-cli --token /path/to/token.jwt artifacts list     # token from a file
```

## Command overview

```text
mender-cli
├── login                       Log in with username/password (optional 2FA)
├── token                       Manage the locally stored auth token
│   ├── set                     Persist a token (e.g. a Personal Access Token)
│   ├── show                    Print the stored token (decoded JWT / JSON / raw)
│   ├── path                    Print the default token storage path
│   └── clear                   Delete the stored token
├── artifacts                   Operations on Mender artifacts
│   ├── list                    List artifacts (filterable)
│   ├── upload                  Upload an artifact
│   ├── download                Download an artifact by id
│   └── delete                  Delete an artifact by id
├── devices                     Operations on Mender devices
│   ├── list                    List devices from device auth
│   ├── get                     Show one device from device auth (by id or filter)
│   └── count                   Count devices from device auth (by status)
├── inventory                   Device inventory (reported attributes and tags)
│   ├── devices
│   │   ├── list                List devices + inventory
│   │   ├── get                 Show one device's inventory (by id or filter)
│   │   └── count               Count devices matching inventory filters
│   ├── device-tags             Manage device tags (list/add/set/delete)
│   └── groups
│       └── list                List inventory static group names
├── terminal                    Remote terminal session on a device
├── port-forward                Forward local ports to a device (TCP/UDP)
├── cp                          Copy files to/from a device
├── version                     Print version and build information
└── completion                  Generate shell completion scripts
```

Run `mender-cli <command> --help` for any command, and see **[CLI.md](./CLI.md)**
for the full reference (flags, parameters and examples per command).

## Configuration file

`mender-cli` supports a JSON configuration file with the `username`, `password`
and `server` parameters. It is looked up, in order, in:

- `/etc/mender-cli/.mender-clirc`
- `$HOME/.mender-clirc`
- `.mender-clirc` (the directory the binary is run from)

Example:

```json
{
    "username": "foo@bar.com",
    "password": "baz",
    "server": "https://hosted.mender.io"
}
```

> All configuration-file parameters can be overridden on the command line.

## Autocompletion

The simplest option installs Bash (and Zsh, if available) completion scripts
into the standard directories:

```console
sudo make install-autocomplete-scripts
```

Alternatively, generate the scripts and source them yourself. `mender-cli` is
built on Cobra, so you can use the standard `completion` command:

```console
mender-cli completion bash > /etc/bash_completion.d/mender-cli
mender-cli completion zsh  > "${fpath[1]}/_mender-cli"
```

The legacy `mender-cli --generate-autocomplete` flag is also supported; it
writes `autocomplete/autocomplete.sh` and `autocomplete/autocomplete.zsh`
relative to the current directory (run it from the source tree).

## Building from source

Requires [Go](https://go.dev/dl/) (see the version in [`go.mod`](./go.mod)).

```console
# Build a binary for the current platform (output: ./mender-cli)
make build

# Build for all supported OS/arch combinations
make build-multiplatform     # linux/darwin/windows × amd64/arm64

# Install into $GOPATH/bin
make install
```

Builds are stamped with version metadata via the linker, surfaced by
`mender-cli version`:

```console
$ mender-cli version
mender-cli, version v1.2.3 (branch: main, revision: abc1234)
  build user:     ci@runner
  build date:     2026-06-09T15:30:32Z
  go version:     go1.24.0
  platform:       linux/amd64
  tags:           nopkcs11
```

Useful quality targets while developing:

```console
make test-unit      # run unit tests
make coverage        # run tests and write coverage.txt
make test-static     # gofmt check + go vet + golangci-lint
make get-go-tools    # install golangci-lint (used by `make lint`)
```

## Repository layout

| Path | Purpose |
| --- | --- |
| `main.go` | Program entry point; delegates to `cmd`. |
| `cmd/` | Cobra command tree — one file per command/subcommand, flag wiring, output formatting, and shared helpers (device targeting, inventory filters, version). |
| `client/` | HTTP/WebSocket clients for the Mender server APIs. |
| `client/useradm/` | Authentication / user administration (login, 2FA, token verification). |
| `client/deployments/` | Artifacts and deployments API. |
| `client/devices/` | Device authentication API. |
| `client/inventory/` | Device inventory and groups API (incl. pagination/count helpers). |
| `client/deviceconnect/` | Device connect API for terminal, port-forward and file transfer over WebSocket. |
| `log/` | Small logging helper used across the tool. |
| `autocomplete/` | Generated Bash/Zsh shell completion scripts. |
| `tests/` | `acceptance/` (containerized CLI tests), `integration/` and supporting `mender_server/` fixtures. |
| `.github/workflows/` | GitHub Actions for CI and releases (see below). |

## CI/CD and releases

CI and releases run on GitHub Actions; the same checks are available locally via
the `make` targets above.

### CI — [`.github/workflows/ci.yml`](./.github/workflows/ci.yml)

Runs on every push (all branches) and on pull requests. Three jobs:

- **Lint & static analysis** — `make gofmt-check`, `make vet`, and
  `golangci-lint` (config in [`.golangci.yml`](./.golangci.yml)).
- **Unit tests & coverage** — `make coverage`, uploads `coverage.txt` as an
  artifact and to Codecov.
- **Cross-compile** — `make build-multiplatform` to verify every target builds.

### Releases — [`.github/workflows/release.yml`](./.github/workflows/release.yml)

Releases are cut **from `master` only** via a manual run:

1. Open **Actions → Release → Run workflow** (from the `master` branch).
2. Choose the version bump: `patch`, `minor` or `major`.

The workflow then:

- computes the next semantic version from the latest `v*` tag,
- creates and pushes the new `vX.Y.Z` tag,
- builds all platforms with the version stamped in,
- packages per-platform `tar.gz` (Linux/macOS) and `zip` (Windows) archives plus
  a `SHA256SUMS.txt`, and
- publishes a GitHub Release with auto-generated notes and the archives attached.

No manual tag creation is needed — the workflow owns tagging.

## Contributing

We welcome your contributions. Please read the
[contributing guide](https://github.com/mendersoftware/mender/blob/master/CONTRIBUTING.md)
to get started.

## License

Mender is licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE)
for the full license text.

## Security disclosure

We take security seriously. If you discover a security issue, please disclose it
by emailing [security@mender.io](mailto:security@mender.io). Please do **not**
create a public issue. Thank you for your cooperation.

## Connect with us

- Join the [Mender Hub discussion forum](https://hub.mender.io)
- Follow us on [Twitter](https://twitter.com/mender_io)
- Fork us on [GitHub](https://github.com/mendersoftware)
- Create an issue in the [bug tracker](https://northerntech.atlassian.net/projects/MEN)
- Email us at [contact@mender.io](mailto:contact@mender.io)
- Connect to the [#mender IRC channel on Libera](https://web.libera.chat/?#mender)
