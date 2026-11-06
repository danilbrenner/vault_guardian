// Command bw-mcp is the Vault Guardian MCP server.
// It speaks MCP over stdin/stdout until the client disconnects.
package main

import (
	"context"
	"errors"
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

	err := server.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
