package registry

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func reset() {
	mu.Lock()
	defer mu.Unlock()
	tools = tools[:0]
	resources = resources[:0]
	prompts = prompts[:0]
}

func dummyToolHandler() ToolHandler {
	return func(_ context.Context, _ Service, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	}
}

func dummyResourceHandler() ResourceHandler {
	return func(_ context.Context, _ Service, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{}, nil
	}
}

func dummyPromptHandler() PromptHandler {
	return func(_ context.Context, _ Service, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{}, nil
	}
}

func TestRegisterAndSnapshot(t *testing.T) {
	reset()

	ts := ToolSpec{
		Tool:    &mcp.Tool{Name: "test_tool"},
		Handler: dummyToolHandler(),
	}
	rs := ResourceSpec{
		Resource: &mcp.Resource{Name: "test_resource", URI: "test://res"},
		Handler:  dummyResourceHandler(),
	}
	ps := PromptSpec{
		Prompt:  &mcp.Prompt{Name: "test_prompt"},
		Handler: dummyPromptHandler(),
	}

	RegisterTool(ts)
	RegisterResource(rs)
	RegisterPrompt(ps)

	tools := Tools()
	resources := Resources()
	prompts := Prompts()

	assert.Len(t, tools, 1)
	assert.Same(t, ts.Tool, tools[0].Tool)
	assert.NotNil(t, tools[0].Handler)

	assert.Len(t, resources, 1)
	assert.Same(t, rs.Resource, resources[0].Resource)
	assert.NotNil(t, resources[0].Handler)

	assert.Len(t, prompts, 1)
	assert.Same(t, ps.Prompt, prompts[0].Prompt)
	assert.NotNil(t, prompts[0].Handler)
}

func TestConcurrentRegistration(t *testing.T) {
	reset()

	var wg sync.WaitGroup
	n := 50
	for i := range n {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			RegisterTool(ToolSpec{
				Tool:    &mcp.Tool{Name: fmt.Sprintf("tool_%d", id)},
				Handler: dummyToolHandler(),
			})
		}(i)
	}
	wg.Wait()

	assert.Len(t, Tools(), n)
}

func TestSnapshotImmutability(t *testing.T) {
	reset()

	RegisterTool(ToolSpec{
		Tool:    &mcp.Tool{Name: "before_snap"},
		Handler: dummyToolHandler(),
	})

	snap := Tools()
	before := len(snap)

	RegisterTool(ToolSpec{
		Tool:    &mcp.Tool{Name: "after_snap"},
		Handler: dummyToolHandler(),
	})

	assert.Len(t, snap, before, "snapshot should not reflect newly registered tools")
	assert.Len(t, Tools(), before+1, "live registry should include the new tool")
}
