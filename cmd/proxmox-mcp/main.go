package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/anomalyco/proxmox-mcp/internal/features"
	"github.com/anomalyco/proxmox-mcp/internal/proxmox"
	"github.com/anomalyco/proxmox-mcp/internal/server"
)

func main() {
	httpAddr := flag.String("http", "", "HTTP listen address (e.g. :8080) for streamable-HTTP transport")
	flag.Parse()

	var cfg proxmox.Config
	if err := cfg.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	svc, err := proxmox.NewService(cfg)
	if err != nil {
		log.Fatalf("failed to create proxmox service: %v", err)
	}

	ctx := context.Background()
	if err := server.Run(ctx, svc, server.Options{
		HTTPAddr: *httpAddr,
	}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
