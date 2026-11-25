# Vault Guardian

Vault Guardian is a personal assistant that keeps an eye on your Bitwarden vault and tells you which passwords need attention.

It never sees or shows your actual passwords. It only looks at things like age and strength, then sends practical recommendations.

You talk to it in Telegram. Mark the accounts that matter most, and it will check them daily and send a weekly report focused on those items.

## MCP server

The MCP server uses Streamable HTTP at `http://localhost:8080/mcp` by default. Run it with `just run-mcp`, which sources `mcp/.env` in the same shell before starting the server. Copy `mcp/.env.example` to `mcp/.env` and set `BW_DATA_DIR` to a writable directory; otherwise bw falls back to `/var/lib/bw`. Set `MCP_ADDR` to change the listen address. Set `BW_DATA_DIR` and `BW_SECRETS_DIR` to override the default Bitwarden data (`/var/lib/bw`) and secret-file (`/run/secrets`) directories. To inspect a running server, use `just inspect-mcp`. The endpoint has no authentication, so keep it on a trusted network and do not expose it publicly.
