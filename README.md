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

## License

MIT
