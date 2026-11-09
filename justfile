# Run the MCP server over Streamable HTTP at http://localhost:8080/mcp.
run-mcp:
    cd mcp && go run ./cmd/bw-mcp

# Start the MCP Inspector web UI with the local MCP server preconfigured.
inspect-mcp:
    npx --yes @modelcontextprotocol/inspector --server-url http://localhost:8080/mcp --transport http

# Run MCP server tests.
test-mcp:
    cd mcp && go test ./...
