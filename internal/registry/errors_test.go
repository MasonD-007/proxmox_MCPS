package registry

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/luthermonson/go-proxmox"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestFailUnauthorized(t *testing.T) {
	result, rerr := Fail("test_op", proxmox.ErrNotAuthorized)
	assert.Nil(t, rerr)
	assert.True(t, result.IsError)
	assertCode(t, result, CodeUnauthorized)
}

func TestFailNotFound(t *testing.T) {
	result, rerr := Fail("test_op", proxmox.ErrNotFound)
	assert.Nil(t, rerr)
	assert.True(t, result.IsError)
	assertCode(t, result, CodeNotFound)
}

func TestFailConflict(t *testing.T) {
	result, rerr := Fail("test_op", errors.New("409 Conflict: resource already exists"))
	assert.Nil(t, rerr)
	assert.True(t, result.IsError)
	assertCode(t, result, CodeConflict)
}

func TestFailUnavailable(t *testing.T) {
	result, rerr := Fail("test_op", proxmox.ErrTimeout)
	assert.Nil(t, rerr)
	assert.True(t, result.IsError)
	assertCode(t, result, CodeUnavailable)
}

func TestFailUnavailable503(t *testing.T) {
	result, rerr := Fail("test_op", errors.New("503 Service Unavailable"))
	assert.Nil(t, rerr)
	assert.True(t, result.IsError)
	assertCode(t, result, CodeUnavailable)
}

func TestFailInternal(t *testing.T) {
	result, rerr := Fail("test_op", errors.New("something unexpected happened"))
	assert.Nil(t, rerr)
	assert.True(t, result.IsError)
	assertCode(t, result, CodeInternal)
}

func TestFailNil(t *testing.T) {
	result, rerr := Fail("test_op", nil)
	assert.Nil(t, rerr)
	assert.True(t, result.IsError)
	assertCode(t, result, CodeInternal)
}

func TestFailNoSensitiveLeak(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"http url", errors.New("GET http://example.com/api?token=abc123 failed")},
		{"auth token", errors.New("auth token=supersecret not valid")},
		{"stack trace", errors.New("stack: goroutine 1 [running]: main.func·1()")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := Fail("op", tc.err)
			var fb failBody
			err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &fb)
			assert.NoError(t, err)
			assert.NotContains(t, fb.Message, "http://")
			assert.NotContains(t, fb.Message, "token=")
			assert.NotContains(t, fb.Message, "goroutine")
		})
	}
}

func assertCode(t *testing.T, result *mcp.CallToolResult, expected FailCode) {
	t.Helper()
	assert.Len(t, result.Content, 1)
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("content is not *mcp.TextContent")
	}
	var fb failBody
	err := json.Unmarshal([]byte(tc.Text), &fb)
	assert.NoError(t, err)
	assert.Equal(t, expected, fb.Code)
}
