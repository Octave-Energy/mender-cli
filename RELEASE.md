# mender-cli octave releases

## v3.2.0

**Date:** 2026-06-10  
**Previous release:** v3.1.0

### Summary

Adds a new top-level `deployments` command group for listing and inspecting
deployments and fetching per-device deployment logs.

### Highlights

- **`mender-cli deployments list`:** List all deployments (auto-paginated).
  Filter with `--id`/`--name` (repeatable), `--status`
  (`inprogress|finished|pending`), `--type` (`software|configuration`),
  `--created-before`/`--created-after` (Unix timestamps), and order with
  `--sort` (`asc|desc`). `-d/--detail` controls verbosity; `-r/--raw` prints the
  server JSON. Shell completion is available for the enum flags.
- **`mender-cli deployments count`:** Return only the total number of
  deployments (read from the server's `X-Total-Count` header), accepting the
  same filters as `list` (except `--sort`).
- **`mender-cli deployments search`:** Find deployments by who they target —
  `--group <name>`, `--device <id>`, or `-f/--filter` (inventory attributes
  resolving to exactly one device, like `devices get --filter`). Matches each
  deployment's declared group/device targeting; `--status`/`--type` narrow the
  scan.
- **`mender-cli deployments get --id <id>`:** Show a single deployment, with
  `-d/--detail` and `-r/--raw`.
- **`mender-cli deployments stats --id <id>`:** Show per-status device counts
  for a deployment.
- **`mender-cli deployments devices --id <id>`:** List a deployment's devices
  and their per-device status (auto-paginated), optionally filtered with
  `--status` (completion supported).
- **`mender-cli deployments log --id <id> --device <device-id>`:** Print a
  device's deployment log as plain text — handy for CI debugging.

### Upgrading

- No breaking changes. Existing commands and flags are unchanged; the
  `deployments` group is purely additive.

---
