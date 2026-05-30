package registry

import (
	"context"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Service interface{}

type (
	ToolHandler     func(ctx context.Context, svc Service, req *mcp.CallToolRequest) (*mcp.CallToolResult, error)
	ResourceHandler func(ctx context.Context, svc Service, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error)
	PromptHandler   func(ctx context.Context, svc Service, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error)
)

type ToolSpec struct {
	Tool    *mcp.Tool
	Handler ToolHandler
}

type ResourceSpec struct {
	Resource        *mcp.Resource
	ResourceTemplate *mcp.ResourceTemplate
	Handler         ResourceHandler
}

type PromptSpec struct {
	Prompt  *mcp.Prompt
	Handler PromptHandler
}

var (
	mu        sync.Mutex
	tools     []ToolSpec
	resources []ResourceSpec
	prompts   []PromptSpec
)

func RegisterTool(s ToolSpec) {
	mu.Lock()
	defer mu.Unlock()
	tools = append(tools, s)
}

func RegisterResource(s ResourceSpec) {
	mu.Lock()
	defer mu.Unlock()
	resources = append(resources, s)
}

func RegisterPrompt(s PromptSpec) {
	mu.Lock()
	defer mu.Unlock()
	prompts = append(prompts, s)
}

func Tools() []ToolSpec {
	mu.Lock()
	defer mu.Unlock()
	r := make([]ToolSpec, len(tools))
	copy(r, tools)
	return r
}

func Resources() []ResourceSpec {
	mu.Lock()
	defer mu.Unlock()
	r := make([]ResourceSpec, len(resources))
	copy(r, resources)
	return r
}

func Prompts() []PromptSpec {
	mu.Lock()
	defer mu.Unlock()
	r := make([]PromptSpec, len(prompts))
	copy(r, prompts)
	return r
}
