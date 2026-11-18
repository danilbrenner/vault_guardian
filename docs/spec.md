# Vault Guardian — Specification v1

## 1. Goal

Vault Guardian is a personal agent that regularly analyzes a Bitwarden vault and provides practical password-hygiene recommendations.

**Core principle:** The system never sees or returns actual passwords. It works exclusively with metadata and derived metrics.

### v1 Scope

- Stateless-ish MCP server (Go) that returns password metrics
- Agent (Node.js) with persistent memory for critical items
- Telegram bot as the sole user interface (long polling)
- Daily check + weekly report
- Two-container deployment for isolation

## 2. Architecture

```
Telegram (long polling)
        ↓
Agent (Node.js) — long-running process
        ↓ HTTP (internal docker network only)
MCP Server (Go) — metrics + cache
        ↓
Bitwarden CLI + HIBP range API
```

- **Two containers** on the same host (Raspberry Pi or similar).
- MCP container holds Bitwarden secrets (API key + master password) via Docker secrets.
- Agent container has no access to Bitwarden credentials or the MCP filesystem.
- Communication between containers is HTTP only, on an internal Docker network. Nothing is published to the host.

## 3. MCP Server (Go)

### 3.1 Responsibilities

- Authenticate and unlock Bitwarden CLI using secrets
- Sync vault on demand
- Compute metrics for every login
- Maintain an in-memory cache
- Expose a small set of HTTP tools
- Never return passwords, TOTP, or other sensitive fields

### 3.2 Bitwarden Interaction Flow

On every cache miss or `force_sync`:

1. `bw status`
2. If `unauthenticated` → `bw login --apikey`
3. If `locked` → `bw unlock` (master password from secret)
4. `bw sync`
5. Read logins, compute metrics, update cache
6. Immediately `bw lock`
7. On error → retry once, then return error to the caller

Session is never kept open longer than necessary.

### 3.3 Caching

- Full metrics are kept in memory after a successful sync.
- Cache is refreshed:
  - On `force_sync`
  - Optionally on a nightly schedule inside the MCP process
- `cache_status` reports age and health of the cache.

### 3.4 Tools

#### `list_logins`

**Parameters:**
- `filter` (optional): `"all"` | `"weak"` | `"pwned"` | `"duplicate"`
- `min_age_days` (optional): number. When set, keep items with `age_days >= min_age_days`. ANDs with `filter`. Omitted means no age constraint. There is no `old` filter and no default.

**Returns:** An object containing an array of login summaries:

```json
{
  "logins": [
    {
      "id": "string",
      "name": "string",
      "masked_username": "string",
      "folder": "string | null",
      "age_days": "number",
      "length": "number",
      "entropy": "number",
      "strength": "VeryWeak | Weak | Moderate | Strong | VeryStrong",
      "pwned_count": "number",
      "hash": "string"
    }
  ]
}
```

#### `get_login`

**Parameters:** `{ "id": "string" }`

**Returns:** Full metrics for a single login (same fields as above).

#### `get_duplicates`

**Parameters:** none (or optional `min_count`)

**Returns:** Groups of logins sharing the same hash:

```json
[
  {
    "hash": "string",
    "count": "number",
    "logins": [
      { "id": "string", "name": "string", "masked_username": "string", "folder": "string | null" }
    ]
  }
]
```

#### `force_sync`

**Parameters:** none

**Returns:**
```json
{
  "synced_at": "ISO8601 timestamp",
  "logins_count": "number"
}
```

#### `cache_status`

**Parameters:** none

**Returns:**
```json
{
  "last_sync": "ISO8601 timestamp",
  "cache_age_minutes": "number",
  "logins_in_cache": "number",
  "bw_status": "unlocked | locked | unauthenticated"
}
```

### 3.5 Password Metrics

#### Entropy
Classical Shannon entropy in bits, calculated from the password and its character set.

#### Strength
Categorical strength based on length + character class presence. Logic follows the rules below (adapted from the reference implementation):

| Condition                                      | Strength     |
|------------------------------------------------|--------------|
| len > 15 + digit + lower + upper + symbol      | VeryStrong   |
| len > 12 + digit + lower + upper + symbol      | Strong       |
| len > 10 + digit + lower + upper + symbol      | Moderate     |
| len > 6  + digit + lower + upper + symbol      | Weak         |
| len > 16 + digit + lower + upper (no symbol)   | VeryStrong   |
| len > 13 + digit + lower + upper               | Strong       |
| len > 10 + digit + lower + upper               | Moderate     |
| len > 6  + digit + lower + upper               | Weak         |
| len > 17 + lower + upper (no digit, no symbol) | VeryStrong   |
| len > 14 + lower + upper                       | Strong       |
| len > 11 + lower + upper                       | Moderate     |
| len > 6  + lower + upper                       | Weak         |
| len > 17 + upper only                          | Strong       |
| len > 13 + upper only                          | Moderate     |
| len > 8  + upper only                          | Weak         |
| len > 17 + lower only                          | Strong       |
| len > 13 + lower only                          | Moderate     |
| len > 8  + lower only                          | Weak         |
| len > 11 + digit only                          | Weak         |
| otherwise                                      | VeryWeak     |

Symbol set: `!@#$%^&*`

#### Duplicate Hash
`SHA-256(password + user_id)`  
`user_id` is a stable identifier for the Bitwarden account (prevents cross-user collisions without storing an extra pepper).

#### Username Masking
Usernames are masked before leaving the MCP (e.g. `d***l@gmail.com`).

## 4. Agent (Node.js)

### 4.1 Responsibilities

- Telegram bot (long polling)
- Persistent memory of critical items
- Scheduling (daily check + weekly report) via in-process cron
- Interpreting MCP metrics and generating recommendations
- Answering free-form user questions

### 4.2 Memory

Simple persistent store (SQLite or JSON file) containing critical items:

```json
{
  "critical": [
    {
      "id": "string",
      "note": "string (optional)",
      "allow_weak": false
    }
  ]
}
```

- Critical items are explicitly marked by the user.
- The agent always reports problems for critical items (no “already told you” suppression).
- User can remove an item from critical or mark it `allow_weak`.

### 4.3 Telegram Commands

| Command          | Description                                      |
|------------------|--------------------------------------------------|
| `/report`        | Full report focused on critical items            |
| `/sync`          | Force MCP cache refresh                          |
| `/status`        | Show MCP cache status                            |
| `/important`     | Mark item as critical (by id or name)            |
| `/forget`        | Remove item from critical                        |
| Free-form text   | Natural language questions about the vault       |

### 4.4 Report Format

```
🔐 Vault Guardian — Report

Total items: 247
Critical: 8

✅ Critical — all good
```

or

```
🔐 Vault Guardian — Report

Total items: 247
Critical: 8

⚠️ Problems in critical (2):

1. GitHub (d***l@gmail.com)
   • Strength: Weak
   • Age: 640 days
   • Pwned: no
   • Recommendation: change

2. AWS Root (a***n@company.com)
   • Strength: Moderate
   • Age: 210 days
   • Pwned: 2
   • Recommendation: change immediately
```

- Only critical items with problems are listed.
- Non-critical items are ignored in v1.
- No automatic suggestions to add new items to critical in v1.

### 4.5 Scheduling

- Daily: lightweight check (can be silent if no new critical problems)
- Weekly: full `/report` pushed to Telegram

## 5. Out of Scope for v1

- Generating new passwords
- Writing anything back to Bitwarden
- Automatic detection / suggestion of newly added passwords
- Proposing items to add to the critical list
- Multi-user support
- Web UI
- Exposing any port to the host

## 6. Deployment Notes

- Two containers, internal Docker network only
- Bitwarden API key + master password supplied to the MCP container via Docker secrets
- Agent container has no secrets and no filesystem access to the MCP container
- Exact orchestration tool (Docker Compose, Ansible, Terraform, etc.) is left open

## 7. Open Implementation Details

- Exact HTTP routes / OpenAPI shape of the MCP
- Choice of persistence for agent memory (SQLite vs JSON)
- Precise nightly sync schedule inside MCP
- Retry/backoff policy details
- Logging and observability

---
*End of v1 specification*
