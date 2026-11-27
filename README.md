# Vault Guardian

Vault Guardian is a personal assistant that keeps an eye on your Bitwarden vault and tells you which passwords need attention.

It never sees or shows your actual passwords. It only looks at things like age and strength, then sends practical recommendations.

You talk to it in Telegram. Mark the accounts that matter most, and it will check them daily and send a weekly report focused on those items.

## Telegram

The bot answers one private chat. Set `ALLOWED_CHAT_ID` to your Telegram user id. In a private chat with the bot, the chat id is your user id.

Find it before the first run:

- Message [@userinfobot](https://t.me/userinfobot) and use the `Id` it replies with.
- Or message your bot, leave Vault Guardian stopped, and read `message.chat.id` from `getUpdates`. Do not run long polling and `getUpdates` at the same time.

```bash
curl -s "https://api.telegram.org/bot$TOKEN/getUpdates"
```

A group or channel has its own id, usually a negative number. Your user id will not match it.

The bot does not start without `ALLOWED_CHAT_ID`. After it is running, a message from any other chat is ignored and that chat id is logged.

Each message in that chat gets one completed reply. The bot does not stream. Set `OPENAI_API_KEY` and `OPENAI_MODEL` in `agent/.env`. Copy `agent/.env.example` to `agent/.env`. The process does not start if either value is missing. It never asks for a password.

## MCP server

The MCP server uses Streamable HTTP at `http://localhost:8080/mcp` by default. Run it with `just run-mcp`, which sources `mcp/.env` in the same shell before starting the server. Copy `mcp/.env.example` to `mcp/.env` and set `BW_DATA_DIR` to a writable directory; otherwise bw falls back to `/var/lib/bw`. Set `MCP_ADDR` to change the listen address. Set `BW_DATA_DIR` and `BW_SECRETS_DIR` to override the default Bitwarden data (`/var/lib/bw`) and secret-file (`/run/secrets`) directories. To inspect a running server, use `just inspect-mcp`. The endpoint has no authentication, so keep it on a trusted network and do not expose it publicly.
