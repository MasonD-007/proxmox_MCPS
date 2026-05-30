package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anomalyco/proxmox-mcp/internal/proxmox"
	"github.com/anomalyco/proxmox-mcp/internal/registry"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Options struct {
	HTTPAddr string
	Logger   *slog.Logger
}

func Run(ctx context.Context, svc *proxmox.Service, opts Options) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	srv := newMCPServer(svc, logger)
	if opts.HTTPAddr != "" {
		return runHTTP(ctx, srv, opts.HTTPAddr, logger)
	}
	return runStdio(ctx, srv, logger)
}

func newMCPServer(svc *proxmox.Service, logger *slog.Logger) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "proxmox-mcp",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Logger: logger,
		Capabilities: &mcp.ServerCapabilities{
			Tools:     &mcp.ToolCapabilities{ListChanged: true},
			Resources: &mcp.ResourceCapabilities{ListChanged: true},
			Prompts:   &mcp.PromptCapabilities{ListChanged: true},
			Logging:   &mcp.LoggingCapabilities{},
		},
	})

	for _, t := range registry.Tools() {
		srv.AddTool(t.Tool, adaptToolHandler(t.Handler, svc))
	}

	for _, r := range registry.Resources() {
		if r.Resource != nil {
			srv.AddResource(r.Resource, adaptResourceHandler(r.Handler, svc))
		}
		if r.ResourceTemplate != nil {
			srv.AddResourceTemplate(r.ResourceTemplate, adaptResourceHandler(r.Handler, svc))
		}
	}

	for _, p := range registry.Prompts() {
		srv.AddPrompt(p.Prompt, adaptPromptHandler(p.Handler, svc))
	}

	logger.Info("server initialized",
		"tools", len(registry.Tools()),
		"resources", len(registry.Resources()),
		"prompts", len(registry.Prompts()),
	)

	return srv
}

func runStdio(ctx context.Context, srv *mcp.Server, logger *slog.Logger) error {
	logger.Info("starting stdio transport")
	err := srv.Run(ctx, &mcp.StdioTransport{})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func runHTTP(ctx context.Context, srv *mcp.Server, addr string, logger *slog.Logger) error {
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return srv
	}, &mcp.StreamableHTTPOptions{
		Logger: logger,
	})

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdownCtx)
	}()

	logger.Info("starting HTTP transport", "addr", addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func adaptToolHandler(h registry.ToolHandler, svc *proxmox.Service) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return h(ctx, svc, req)
	}
}

func adaptResourceHandler(h registry.ResourceHandler, svc *proxmox.Service) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return h(ctx, svc, req)
	}
}

func adaptPromptHandler(h registry.PromptHandler, svc *proxmox.Service) mcp.PromptHandler {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return h(ctx, svc, req)
	}
}
