// Command bw-mcp is the Vault Guardian MCP server.
// It serves MCP over Streamable HTTP.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"bw-mcp/internal/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "bw-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return server.Run(ctx)
}
