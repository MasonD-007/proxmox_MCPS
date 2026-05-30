package deps

import (
	_ "github.com/luthermonson/go-proxmox"
	_ "github.com/modelcontextprotocol/go-sdk/mcp"
	_ "go.uber.org/zap"

	_ "github.com/stretchr/testify/assert"
	_ "go.uber.org/goleak"
	_ "gopkg.in/h2non/gock.v1"
)
