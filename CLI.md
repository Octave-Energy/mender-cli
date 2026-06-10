# mender-cli command reference

Detailed reference for every `mender-cli` command: synopsis, flags and
examples. For an overview, authentication and build instructions see
[README.md](./README.md).

Every command help is also available at runtime:

```console
mender-cli --help
mender-cli <command> --help
```

## Global flags

These flags are accepted by every command (they are inherited from the root
command).

| Flag | Description |
| --- | --- |
| `--server <url>` | Root server URL (default `https://hosted.mender.io`). If the scheme is omitted, `https://` is assumed. |
| `-k, --skip-verify` | Skip TLS certificate verification (useful for self-signed test servers). |
| `--token <path>` | Path to a file containing a JWT token to use for this invocation. |
| `--token-value <jwt>` | JWT token value (e.g. an API key) to use for this invocation. |
| `-v, --verbose` | Print verbose output. |
| `-h, --help` | Show help for the command. |

Most flags can also be supplied through the [configuration file](./README.md#configuration-file).

## Command tree

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
│   ├── device-tags             Manage the tags set on a device
│   │   ├── list                List all tags set on a device
│   │   ├── add                 Add a new tag to a device
│   │   ├── set                 Change the value of an existing tag
│   │   └── delete              Delete a tag from a device
│   └── groups
│       └── list                List inventory static group names
├── terminal                    Remote terminal session on a device
├── port-forward                Forward local ports to a device (TCP/UDP)
├── cp                          Copy files to/from a device
├── version                     Print version and build information
└── completion                  Generate shell completion scripts
```

## Device targeting

`terminal`, `port-forward`, `cp`, `devices get`, and `inventory devices get`
target a single device. You can identify the device in one of these ways:

- `--id <device-id>` — use the device id verbatim.
- `-f, --filter <expr>` — an inventory filter expression that must match
  **exactly one** device. If multiple devices match, the matching ids are
  listed so you can refine the query.
- For `terminal` and `port-forward` a positional `DEVICE_ID` is also accepted
  for backwards compatibility.

Filter expressions use the form `name=value` or `scope/name=value` (the scope
defaults to `inventory`). Repeat `-f` for multiple filters; they are ANDed.

---

## login

Log in to the Mender server and store a session token for subsequent commands.
Required before other operations unless you authenticate with a token instead
(see [`token set`](#token-set)).

```console
mender-cli login [flags]
```

| Flag | Description |
| --- | --- |
| `--username <email>` | Username (email). Prompted if not provided. |
| `--password <pass>` | Password. Prompted (masked) if not provided. |
| `--2fa-code <code>` | Two-factor authentication token, if 2FA is enabled. |

```console
mender-cli --server https://hosted.mender.io login --username me@example.com
mender-cli login --username me@example.com --password secret
mender-cli login --username me@example.com --2fa-code 123456
```

---

## token

Manage the locally stored authentication token without needing to know its
on-disk location. Useful for [Personal Access Tokens (PATs)](./README.md#authentication).

### token set

Persist a token in the local store used by all other commands. The token is
read from stdin when piped, or via a masked interactive prompt otherwise. After
saving, the token is validated against the configured server; a failed
validation only produces a warning — the token is always saved.

```console
mender-cli token set
echo "$MY_PAT" | mender-cli token set
```

### token show

Print the locally stored token. By default the JWT header and payload are
rendered as a human-readable table with friendly claim labels and RFC3339
timestamps for the standard date claims (`exp`, `iat`, `nbf`, `auth_time`); the
signature segment is omitted.

| Flag | Description |
| --- | --- |
| `--json` | Print the decoded JWT header and payload as JSON. |
| `--raw` | Print the unparsed token verbatim (useful for piping). |

```console
mender-cli token show
mender-cli token show --json
mender-cli token show --raw
```

### token path

Print the absolute filesystem path where the token is stored by default. Always
reflects the default platform-specific location and ignores any `--token`
override.

```console
mender-cli token path
```

### token clear

Delete the locally stored token. An interactive confirmation is required by
default; in non-interactive contexts `--yes` is mandatory.

| Flag | Description |
| --- | --- |
| `-y, --yes` | Do not prompt for confirmation before deleting. |

```console
mender-cli token clear
mender-cli token clear --yes
```

---

## artifacts

Operations on Mender artifacts.

### artifacts list

List artifacts from the Mender server.

The `--name`, `--description` and `--device-type` filters support **prefix
matching** by appending `*` (e.g. `--name 'my-app*'`). `--name` may be
**repeated** to match several exact names, but a prefix match cannot be combined
with multiple names.

| Flag | Description |
| --- | --- |
| `-d, --detail <0..3>` | Detail level of the output. |
| `--name <name>` | Filter by artifact name; append `*` for prefix matching; repeat to match several names. |
| `--description <text>` | Filter by artifact description; append `*` for prefix matching. |
| `--device-type <type>` | Filter by compatible device type; append `*` for prefix matching. |
| `-r, --raw` | Print the raw JSON returned by the server. |

```console
mender-cli artifacts list
mender-cli artifacts list --detail 3
mender-cli artifacts list --name my-app --device-type raspberrypi4
mender-cli artifacts list --name 'release-*'
mender-cli artifacts list --name app-a --name app-b
mender-cli artifacts list --raw
```

### artifacts upload

Upload a Mender artifact (`.mender` file) to the server.

```console
mender-cli artifacts upload [flags] ARTIFACT
```

| Flag | Description |
| --- | --- |
| `--description <text>` | Artifact description. |
| `--direct` | Upload directly to storage. |
| `--no-progress` | Disable the progress bar. |

```console
mender-cli artifacts upload ./release-1.mender
mender-cli artifacts upload --description "release 1" ./release-1.mender
```

### artifacts download

Download a Mender artifact by id.

```console
mender-cli artifacts download [flags] ARTIFACT_ID
```

| Flag | Description |
| --- | --- |
| `--destination-path <dir>` | Destination path to download to. |
| `--no-progress` | Disable the progress bar. |

```console
mender-cli artifacts download 0123456789abcdef0123456789abcdef
mender-cli artifacts download 0123456789abcdef0123456789abcdef --destination-path ./downloads
```

### artifacts delete

Delete a Mender artifact by id.

```console
mender-cli artifacts delete ARTIFACT_ID
```

```console
mender-cli artifacts delete 0123456789abcdef0123456789abcdef
```

---

## devices

Operations on Mender devices.

### devices list

List devices from the Mender server's device authentication service. Use
`--status` to filter by authentication status.

| Flag | Description |
| --- | --- |
| `-d, --detail <0..3>` | Detail level of the output. |
| `--status <status>` | Only devices with this auth status: `pending`, `accepted`, `rejected`, `preauthorized`, `noauth` (shell completion supported). |
| `-r, --raw` | Print the raw JSON returned by the server. |

```console
mender-cli devices list
mender-cli devices list --detail 3
mender-cli devices list --status pending
mender-cli devices list --raw
```

### devices get

Show a single device from the Mender server's device authentication service.
Specify the target with either `--id` or a `--filter` expression that matches
exactly one device (see [Device targeting](#device-targeting)).

| Flag | Description |
| --- | --- |
| `--id <device-id>` | Device id to target (verbatim). |
| `-f, --filter <expr>` | Filter by attribute; must match exactly one device. |
| `-d, --detail <0..3>` | Detail level of the output. |
| `-r, --raw` | Print the raw JSON returned by the server. |

```console
mender-cli devices get --id 0123456789abcdef0123456789abcdef
mender-cli devices get --id 0123456789abcdef0123456789abcdef --detail 3
mender-cli devices get -f hostname=my-gateway
mender-cli devices get -f inventory/mac=00:11:22:33:44:55 -f tags/env=prod
```

### devices count

Count devices from the Mender server's device authentication service. This
efficiently returns only the total number of devices without listing them. Use
`--status` to count only devices with a given authentication status.

| Flag | Description |
| --- | --- |
| `--status <status>` | Only devices with this auth status: `pending`, `accepted`, `rejected`, `preauthorized`, `noauth` (shell completion supported). |

```console
mender-cli devices count
mender-cli devices count --status pending
```

---

## inventory

Device inventory: reported attributes and tags.

### inventory devices list

Get devices and their reported inventory (attributes, tags). Use `--filter` to narrow the results.

| Flag | Description |
| --- | --- |
| `-d, --detail <0..3>` | Inventory detail level. |
| `-f, --filter <expr>` | Filter by attribute: `name=value` or `scope/name=value`; repeat for multiple. |
| `--group <name>` | Only devices in this group. |
| `--has-group` | Only devices in a group (`true`) or not in any group (`false`); omit for both. |
| `-r, --raw` | Output the raw JSON returned by the server. |
| `--sort <expr>` | Sort by attributes, e.g. `attr1:asc,attr2:desc`. |

```console
mender-cli inventory devices list
mender-cli inventory devices list --raw
mender-cli inventory devices list -f hostname=my-gateway -d 1
mender-cli inventory devices list -f inventory/mac=00:11:22:33:44:55 -f tags/env=prod
mender-cli inventory devices list --group production --has-group=true
```

### inventory devices get

Show the reported inventory (attributes, tags) of a single device. Specify the
target with either `--id` or a `--filter` expression that matches exactly one
device (see [Device targeting](#device-targeting)).

| Flag | Description |
| --- | --- |
| `--id <device-id>` | Device id to target (verbatim). |
| `-f, --filter <expr>` | Filter by attribute; must match exactly one device. |
| `-d, --detail <0..3>` | Inventory detail level. |
| `-r, --raw` | Output the raw JSON returned by the server. |

```console
mender-cli inventory devices get --id 0123456789abcdef0123456789abcdef
mender-cli inventory devices get --id 0123456789abcdef0123456789abcdef --raw
mender-cli inventory devices get -f hostname=my-gateway
mender-cli inventory devices get -f inventory/mac=00:11:22:33:44:55 -f tags/env=prod
```

### inventory devices count

Count devices matching the given inventory filters. Efficiently returns only
the total number of matching devices (read from the server's `X-Total-Count`
header) without listing them.

| Flag | Description |
| --- | --- |
| `-f, --filter <expr>` | Filter by attribute: `name=value` or `scope/name=value`; repeat for multiple. |
| `--group <name>` | Only devices in this group. |
| `--has-group` | Only devices in a group (`true`) or not in any group (`false`); omit for both. |

```console
mender-cli inventory devices count
mender-cli inventory devices count -f hostname=my-gateway
mender-cli inventory devices count -f inventory/mac=00:11:22:33:44:55 -f tags/env=prod
mender-cli inventory devices count --group production --has-group=true
```

### inventory groups list

List inventory static group names.

| Flag | Description |
| --- | --- |
| `-r, --raw` | Output the raw JSON returned by the server. |
| `--status <status>` | Only groups for devices with this auth set status. |

```console
mender-cli inventory groups list
mender-cli inventory groups list --raw
mender-cli inventory groups list --status accepted --raw
```

### inventory device-tags

Manage the tags (tags-scope inventory attributes) on a single device. Every
subcommand targets one device with `--id` or a `--filter` expression that
matches exactly one device (see [Device targeting](#device-targeting)).

Tag *reading* is also available through `inventory devices get`/`list` (tags
appear with a `[tags]` scope prefix); this group adds tag mutation.

| Subcommand | Description |
| --- | --- |
| `list` | List all tags set on the device (`-r/--raw` for JSON). |
| `add NAME=VALUE` | Add a new tag. Fails if the tag already exists. |
| `set NAME=VALUE` | Change an existing tag's value. Fails if the tag does not exist. |
| `delete NAME` | Delete a tag. Fails if the tag is not set. |

`add` and `set` accept an optional `--description <text>` for the tag's
human-readable description. On `set`, omitting `--description` preserves the
existing description; passing it (even empty, `--description ""`) overwrites it.

```console
mender-cli inventory device-tags list --id 0123456789abcdef0123456789abcdef
mender-cli inventory device-tags add environment=production --id 0123456789abcdef0123456789abcdef
mender-cli inventory device-tags add owner=ops --description "owning team" --id 0123456789abcdef0123456789abcdef
mender-cli inventory device-tags set environment=staging -f hostname=my-gateway
mender-cli inventory device-tags delete environment --id 0123456789abcdef0123456789abcdef
```

Writes use the device's `ETag` for optimistic concurrency: if the device's tags
change between the read and the write, the command fails with a conflict error
and can be retried.

---

## terminal

Remotely access a terminal on a device. The target device can be selected with
a positional `DEVICE_ID`, the `--id` flag, or a `--filter` expression that
matches exactly one device. The session can be recorded locally, and a recorded
session can be played back without connecting to any device.

```console
mender-cli terminal [DEVICE_ID] [flags]
```

| Flag | Description |
| --- | --- |
| `--id <device-id>` | Device id to target (verbatim). |
| `-f, --filter <expr>` | Filter by attribute; must match exactly one device. |
| `--record <file>` | Recording file path to save the session to. |
| `--playback <file>` | Recording file path to play back from (no device required). |

```console
mender-cli terminal 0123456789abcdef0123456789abcdef
mender-cli terminal --id 0123456789abcdef0123456789abcdef
mender-cli terminal -f hostname=my-gateway
mender-cli terminal 0123456789abcdef0123456789abcdef --record session.rec
mender-cli terminal --playback session.rec
```

---

## port-forward

Forward one or more local ports to remote port(s) on the device. Supports both
TCP and UDP.

A port specification can be prefixed with `tcp/` or `udp/`; without a prefix,
TCP is the default. `REMOTE_PORT` may also be given as `REMOTE_HOST:REMOTE_PORT`
to forward to a third host reachable from the device's network (the spec then
becomes `LOCAL_PORT:REMOTE_HOST:REMOTE_PORT`). Multiple specifications can be
given.

```console
mender-cli port-forward DEVICE_ID [tcp|udp/]LOCAL_PORT[:REMOTE_PORT] [...] [flags]
```

| Flag | Description |
| --- | --- |
| `--id <device-id>` | Device id to target (verbatim). |
| `-f, --filter <expr>` | Filter by attribute; must match exactly one device. |
| `--bind <host>` | Binding host (default `127.0.0.1`). |

```console
mender-cli port-forward DEVICE_ID 8000:8000
mender-cli port-forward DEVICE_ID udp/8000:8000
mender-cli port-forward DEVICE_ID tcp/8000:192.168.1.1:8000
mender-cli port-forward --id DEVICE_ID 8000:8000
mender-cli port-forward -f hostname=my-gateway 8000:8000
```

---

## cp

Copy files to or from a connected device. Specify the direction explicitly with
`--upload` or `--download`, and target the device with `--id` or `--filter`.

| Flag | Description |
| --- | --- |
| `--id <device-id>` | Device id to target (verbatim). |
| `-f, --filter <expr>` | Filter by attribute; must match exactly one device. |
| `--upload` | Copy from the local host to the device. |
| `--download` | Copy from the device to the local host. |
| `--source-path <path>` | Source file path. |
| `--dest-path <path>` | Destination file path. |

```console
mender-cli cp --id DEVICE_ID --upload --source-path ./app.conf --dest-path /etc/app.conf
mender-cli cp --id DEVICE_ID --download --source-path /var/log/syslog --dest-path ./syslog
mender-cli cp --filter hostname=my-gateway --download --source-path /etc/hosts --dest-path ./hosts
```

---

## version

Print the version together with build metadata: git branch and revision, build
user and date, the Go version and platform it was built for, and the build tags
used. These values are injected at build time (see
[Building from source](./README.md#building-from-source)).

| Flag | Description |
| --- | --- |
| `--short` | Print only the one-line version summary. |

```console
mender-cli version
mender-cli version --short
```

---

## completion

Generate a shell autocompletion script (Bash, Zsh, Fish, PowerShell). See
[Autocompletion](./README.md#autocompletion) in the README for installation
details.

```console
mender-cli completion bash
mender-cli completion zsh
```
