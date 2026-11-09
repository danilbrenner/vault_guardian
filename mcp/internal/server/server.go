// Package server builds the Vault Guardian MCP server.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"bw-mcp/internal/bw"
	"bw-mcp/internal/cache"
	"bw-mcp/internal/secrets"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	name           = "bw-mcp"
	title          = "Vault Guardian"
	version        = "0.1.0"
	address        = ":8080"
	path           = "/mcp"
	defaultDataDir = "/var/lib/bw"
	instructions   = "Vault Guardian reports metadata for a local Bitwarden vault. Tools return item id, name, folder, and age in days. They do not return passwords."
)

// New returns a configured MCP server. logger and store may be nil.
// A nil store leaves the vault tools registered; calls then return an error.
func New(logger *slog.Logger, store *cache.Cache) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:        name,
		Title:       title,
		Version:     version,
		Description: "Local MCP server for Vault Guardian.",
	}, &mcp.ServerOptions{
		Instructions: instructions,
		Logger:       logger,
	})
	registerTools(s, store)
	return s
}

// Run serves the server over Streamable HTTP until ctx is cancelled.
func Run(ctx context.Context) error {
	secs, err := secrets.Load("")
	if err != nil {
		return err
	}
	// The data directory is private to this container and must already exist.
	cli := &bw.CLI{Runner: bw.ExecRunner{}, DataDir: defaultDataDir}
	store := cache.New(cli, secs, time.Now)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	addr := os.Getenv("MCP_ADDR")
	if addr == "" {
		addr = address
	}

	mcpServer := New(logger, store)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return mcpServer
	}, &mcp.StreamableHTTPOptions{Logger: logger})
	mux := http.NewServeMux()
	mux.Handle(path, handler)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- httpServer.ListenAndServe()
	}()
	logger.Info("MCP server listening", "address", addr, "endpoint", path)

	select {
	case err := <-serverErr:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-serverErr; err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}
