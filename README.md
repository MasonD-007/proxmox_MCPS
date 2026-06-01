# proxmox-mcp

MCP server for Proxmox VE — manage VMs, containers, snapshots, and cluster resources through the Model Context Protocol.

## Quick start

```shell
go build ./...
./proxmox-mcp
```

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `PROXMOX_HOST` | yes | Proxmox VE host URL |
| `PROXMOX_TOKEN_ID` | yes | API token ID |
| `PROXMOX_TOKEN_SECRET` | yes | API token secret |
| `PROXMOX_INSECURE_TLS` | no | Skip TLS verification (default: `false`) |

## Development

Before opening a pull request, ensure your branch passes:

```shell
go vet ./...
go test -race ./...
go build ./...
```

The repository uses GitHub Actions to enforce these checks. Feature files in
`internal/features/` must prefix their package-level identifiers with the
filename token (e.g. identifiers in `snapshot.go` must contain `snapshot`).
The `init` function is exempt. This is enforced by an AST-based test in
`internal/lint/featureprefix_test.go`.

## License

MIT
