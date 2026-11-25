# Run the MCP server over Streamable HTTP at http://localhost:8080/mcp.
# Source mcp/.env in this shell so BW_DATA_DIR is set before the process starts.
run-mcp:
    cd mcp && if [ -f .env ]; then set -a && . .env && set +a && echo "loaded .env BW_DATA_DIR=$BW_DATA_DIR"; else echo ".env not found; BW_DATA_DIR defaults to /var/lib/bw" >&2; fi  && go run ./cmd/bw-mcp

# Start the MCP Inspector web UI with the local MCP server preconfigured.
inspect-mcp:
    npx --yes @modelcontextprotocol/inspector --server-url http://localhost:8080/mcp --transport http

# Run MCP server tests.
test-mcp:
    cd mcp && go test ./...
