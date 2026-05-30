package server

import (
	"context"
	"encoding/json"
	"iter"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/proxmox-mcp/internal/proxmox"
	"github.com/anomalyco/proxmox-mcp/internal/registry"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var registerMu sync.Mutex

func withTestItems(fn func()) {
	registerMu.Lock()
	defer registerMu.Unlock()

	registry.RegisterTool(registry.ToolSpec{
		Tool: &mcp.Tool{
			Name:        "test_noop",
			Description: "A no-op test tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		Handler: func(ctx context.Context, svc registry.Service, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			}, nil
		},
	})

	registry.RegisterResource(registry.ResourceSpec{
		Resource: &mcp.Resource{
			Name: "test-resource",
			URI:  "test://resource",
		},
		Handler: func(ctx context.Context, svc registry.Service, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: req.Params.URI, Text: "resource data"}},
			}, nil
		},
	})

	registry.RegisterPrompt(registry.PromptSpec{
		Prompt: &mcp.Prompt{
			Name: "test-prompt",
		},
		Handler: func(ctx context.Context, svc registry.Service, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: "hello"}},
				},
			}, nil
		},
	})

	fn()
}

func testService(t *testing.T) *proxmox.Service {
	t.Helper()
	svc, err := proxmox.NewService(proxmox.Config{
		Host:        "test",
		TokenID:     "dummy",
		TokenSecret: "dummy",
	})
	require.NoError(t, err)
	return svc
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestServerListsAdvertisedTools(t *testing.T) {
	withTestItems(func() {
		svc := testService(t)
		srv := newMCPServer(svc, testLogger(t))

		t1, t2 := mcp.NewInMemoryTransports()
		_, err := srv.Connect(context.Background(), t1, nil)
		require.NoError(t, err)

		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.1.0"}, nil)
		cs, err := client.Connect(context.Background(), t2, nil)
		require.NoError(t, err)
		defer cs.Close()

		tools := collectTools(t, cs.Tools(context.Background(), nil))
		var names []string
		for _, tt := range tools {
			names = append(names, tt.Name)
		}
		assert.Contains(t, names, "test_noop")
	})
}

func TestServerListsAdvertisedResources(t *testing.T) {
	withTestItems(func() {
		svc := testService(t)
		srv := newMCPServer(svc, testLogger(t))

		t1, t2 := mcp.NewInMemoryTransports()
		_, err := srv.Connect(context.Background(), t1, nil)
		require.NoError(t, err)

		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.1.0"}, nil)
		cs, err := client.Connect(context.Background(), t2, nil)
		require.NoError(t, err)
		defer cs.Close()

		resources := collectResources(t, cs.Resources(context.Background(), nil))
		var names []string
		for _, r := range resources {
			names = append(names, r.Name)
		}
		assert.Contains(t, names, "test-resource")
	})
}

func TestServerListsAdvertisedPrompts(t *testing.T) {
	withTestItems(func() {
		svc := testService(t)
		srv := newMCPServer(svc, testLogger(t))

		t1, t2 := mcp.NewInMemoryTransports()
		_, err := srv.Connect(context.Background(), t1, nil)
		require.NoError(t, err)

		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.1.0"}, nil)
		cs, err := client.Connect(context.Background(), t2, nil)
		require.NoError(t, err)
		defer cs.Close()

		prompts := collectPrompts(t, cs.Prompts(context.Background(), nil))
		var names []string
		for _, p := range prompts {
			names = append(names, p.Name)
		}
		assert.Contains(t, names, "test-prompt")
	})
}

func TestCallNoopTool(t *testing.T) {
	withTestItems(func() {
		svc := testService(t)
		srv := newMCPServer(svc, testLogger(t))

		t1, t2 := mcp.NewInMemoryTransports()
		_, err := srv.Connect(context.Background(), t1, nil)
		require.NoError(t, err)

		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.1.0"}, nil)
		cs, err := client.Connect(context.Background(), t2, nil)
		require.NoError(t, err)
		defer cs.Close()

		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "test_noop",
			Arguments: map[string]any{},
		})
		require.NoError(t, err)
		require.False(t, result.IsError)
		require.Len(t, result.Content, 1)
		tc, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok, "content is not *mcp.TextContent")
		assert.Equal(t, "ok", tc.Text)
	})
}

func TestRunStdioCancelsCleanly(t *testing.T) {
	svc := testService(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, svc, Options{Logger: testLogger(t)})
	}()

	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not shut down within timeout")
	}
}

func collectTools(t *testing.T, seq iter.Seq2[*mcp.Tool, error]) []*mcp.Tool {
	t.Helper()
	var tools []*mcp.Tool
	for tt, err := range seq {
		require.NoError(t, err)
		tools = append(tools, tt)
	}
	return tools
}

func collectResources(t *testing.T, seq iter.Seq2[*mcp.Resource, error]) []*mcp.Resource {
	t.Helper()
	var resources []*mcp.Resource
	for r, err := range seq {
		require.NoError(t, err)
		resources = append(resources, r)
	}
	return resources
}

func collectPrompts(t *testing.T, seq iter.Seq2[*mcp.Prompt, error]) []*mcp.Prompt {
	t.Helper()
	var prompts []*mcp.Prompt
	for p, err := range seq {
		require.NoError(t, err)
		prompts = append(prompts, p)
	}
	return prompts
}
