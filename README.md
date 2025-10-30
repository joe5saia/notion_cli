# notionctl

`notionctl` is a Go CLI that wraps the latest Notion API to help you inspect data sources, capture changes, edit pages, and append Markdown content directly from your terminal.

The implementation targets Go 1.22 and emphasises reproducible workflows and strict linting/testing.

## Getting Started

1. Install Go 1.22 (or newer).  
2. Clone the repository and install toolchain binaries:

   ```sh
   git clone https://github.com/yourorg/notionctl.git
   cd notionctl
   make tools
   ```

3. Build the CLI (optional if you run through `go run`):

   ```sh
   go build ./...
   ```

4. Export your Notion integration token (or use the `auth login` command described below).

## Authentication

`notionctl` stores tokens in the OS keyring and persists auxiliary configuration under `~/.config/notionctl`.

```sh
# Provide an existing integration token interactively (password prompt is hidden)
notionctl auth login --token "secret_xxx"

# Use a different profile (defaults to "default")
notionctl auth login --profile personal --token "secret_xxx"
```

Tokens are retrieved implicitly by every command via the selected profile (`--profile` flag).

## Commands

### Data Sources

```sh
# List data sources under a database container
notionctl ds list --database-id 12345678abcd

# Query a data source with optional relation expansion
notionctl ds query \
  --data-source-id abcdef012345 \
  --filter-properties Name,Status \
  --expand Assignee,Dependencies \
  --format table
```

### Changes

Inspect edits within a time window (UTC timestamps, RFC3339):

```sh
notionctl changes \
  --data-source-id abcdef012345 \
  --since 2025-10-01T00:00:00Z \
  --until 2025-10-07T23:59:59Z \
  --expand Assignee \
  --format json
```

### Pages

```sh
# Retrieve a page (with optional relation expansion)
notionctl pages get 1234abcd --expand Assignee --format table

# Update properties from JSON (relations are merged, not replaced, unless --replace-relations is used)
cat > props.json <<'JSON'
{
  "Status": { "status": { "name": "In Progress" } },
  "Tags":   { "multi_select": [ { "name": "CLI" } ] },
  "Dependencies": { "relation": [ { "id": "deadbeef1234" } ] }
}
JSON

notionctl pages update 1234abcd --props props.json
```

### Blocks

```sh
# Append Markdown to a page or block
notionctl blocks append 1234abcd --md ./notes.md
```

The Markdown converter supports headings, lists, code blocks, callouts, and other common elements via [`notionmd`](https://github.com/brittonhayes/notionmd).

### Sync

Watch for webhook deliveries with a polling fallback to keep local consumers up to date:

```sh
notionctl sync watch \
  --data-source-id abcdef012345 \
  --listen :8914 \
  --webhook-secret "$NOTION_WEBHOOK_SECRET" \
  --poll-interval 2m
```

The watcher acknowledges Notion deliveries, verifies the shared secret when provided, and emits JSON events for both webhook payloads (`{"kind":"webhook", ...}`) and periodic change sweeps (`{"kind":"poll", ...}`). Use `--no-webhook` to rely solely on polling and `--suppress-empty` to omit idle poll outputs.

## Tooling & Quality Gates

- Formatting is enforced by [`gofumpt`](https://github.com/mvdan/gofumpt); run `make fmt` to rewrite files and `make fmt-check` to verify formatting.
- Static analysis is handled by [`golangci-lint`](https://github.com/golangci/golangci-lint) with a strict ruleset (`.golangci.yml`). Run `make lint` to execute the full suite.
- `make ci-lint` runs both format checks and linting.

All targets live in the project `Makefile`.

## Testing

Run the full test suite (unit tests and lightweight command helpers):

```sh
go test ./...
```

Notable coverage:

- `internal/notion` – retry/backoff and header behaviours
- `internal/expand` – relation expansion caching and concurrency
- `cmd/blocks` / `cmd/changes` – Markdown conversion and filter generation

## Working With Profiles

Use profiles to separate different integrations/workspaces:

```sh
notionctl auth login --profile work --token "secret_work"
notionctl ds list --profile work --database-id ...
```

Each profile creates a separate token entry in the keyring and a dedicated Notion-Version preference in `~/.config/notionctl/config.yaml`.

## Contributing

1. Run `go test ./...` and `make lint` before submitting changes.
2. Keep README usage examples current when new commands or flags are added.
3. Update or extend the test suite alongside functional changes.

Pull requests are reviewed for correctness, documentation updates, and compliance with the linting suite.
