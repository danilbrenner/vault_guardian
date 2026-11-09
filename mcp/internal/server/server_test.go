package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerRespondsToPing(t *testing.T) {
	s := New(slog.New(slog.DiscardHandler), nil)

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := s.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("connect server: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := make([]string, len(tools.Tools))
	for i, tool := range tools.Tools {
		names[i] = tool.Name
	}
	slices.Sort(names)
	if strings.Join(names, ",") != "cache_status,force_sync,get_item,list_items,ping" {
		t.Fatalf("tools = %#v", names)
	}
	if init := session.InitializeResult(); init == nil || strings.Contains(init.Instructions, "No vault tools") {
		t.Fatalf("instructions = %#v", init)
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "ping"})
	if err != nil {
		t.Fatalf("call ping: %v", err)
	}
	if res.IsError {
		t.Fatalf("ping returned a tool error: %#v", res)
	}
}

func TestHTTPServerRespondsToPing(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	server := New(logger, nil)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Logger: logger})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(httpServer.Close)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "ping"})
	if err != nil {
		t.Fatalf("call ping: %v", err)
	}
	if res.IsError {
		t.Fatalf("ping returned a tool error: %#v", res)
	}
}
