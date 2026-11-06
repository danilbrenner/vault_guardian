# Run the MCP server over stdio.
run-mcp:
    cd mcp && go run ./cmd/bw-mcp

# Open MCP Inspector against the local server.
inspect-mcp:
    cd mcp && npx --yes @modelcontextprotocol/inspector go run ./cmd/bw-mcp

# Run MCP server tests.
test-mcp:
    cd mcp && go test ./...
