// Package server builds the Vault Guardian MCP server.
//
// The server speaks MCP over stdio. Stdout is reserved for protocol
// messages, so all logging must go to stderr.
package server

import (
	"context"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	name    = "bw-mcp"
	title   = "Vault Guardian"
	version = "0.1.0"
)

// New returns a configured MCP server. logger may be nil.
func New(logger *slog.Logger) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:        name,
		Title:       title,
		Version:     version,
		Description: "Local MCP server for Vault Guardian.",
	}, &mcp.ServerOptions{
		Instructions: "Vault Guardian MCP server. No vault tools are registered yet.",
		Logger:       logger,
	})
	registerTools(s)
	return s
}

// Run serves the server on stdin/stdout until the client disconnects or ctx is cancelled.
func Run(ctx context.Context) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	return New(logger).Run(ctx, &mcp.StdioTransport{})
}

type pingResult struct {
	Status string `json:"status" jsonschema:"ok when the server is running"`
}

func registerTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ping",
		Description: "Check that the Vault Guardian MCP server is running.",
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, pingResult, error) {
		return nil, pingResult{Status: "ok"}, nil
	})
}
