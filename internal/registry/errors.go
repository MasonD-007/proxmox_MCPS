package registry

import (
	"encoding/json"
	"strings"

	"github.com/luthermonson/go-proxmox"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type FailCode string

const (
	CodeNotFound     FailCode = "not_found"
	CodeUnauthorized FailCode = "unauthorized"
	CodeConflict     FailCode = "conflict"
	CodeUnavailable  FailCode = "unavailable"
	CodeInternal     FailCode = "internal"
)

type failBody struct {
	Code    FailCode `json:"code"`
	Message string   `json:"message"`
}

func Fail(op string, err error) (*mcp.CallToolResult, error) {
	code := classify(err)
	msg := cleanMessage(op, err)

	body, _ := json.Marshal(failBody{Code: code, Message: msg})

	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(body)},
		},
	}, nil
}

func classify(err error) FailCode {
	if err == nil {
		return CodeInternal
	}
	switch {
	case proxmox.IsNotAuthorized(err):
		return CodeUnauthorized
	case proxmox.IsNotFound(err):
		return CodeNotFound
	case isConflict(err):
		return CodeConflict
	case isUnavailable(err):
		return CodeUnavailable
	default:
		return CodeInternal
	}
}

func isConflict(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "409") || strings.Contains(msg, "Conflict")
}

func isUnavailable(err error) bool {
	if proxmox.IsTimeout(err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "503") || strings.Contains(msg, "unavailable")
}

func cleanMessage(op string, err error) string {
	if err == nil {
		return op + ": unknown error"
	}
	msg := err.Error()
	if strings.Contains(msg, "http") || strings.Contains(msg, "token") || strings.Contains(msg, "stack") {
		msg = "internal error"
	}
	return op + ": " + msg
}
