// Required configuration is read from the process environment.
// dotenv-cli loads .env before the process starts. This module does not.

const INTEGER = /^-?\d+$/;

export class ConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ConfigError";
  }
}

export function telegramToken(env: NodeJS.ProcessEnv): string {
  const raw = env.TELEGRAM_BOT_TOKEN;
  if (raw === undefined || raw.trim() === "") {
    throw new ConfigError("TELEGRAM_BOT_TOKEN is required");
  }
  return raw.trim();
}

export function allowedChatId(env: NodeJS.ProcessEnv): number {
  const raw = env.ALLOWED_CHAT_ID;
  if (raw === undefined || raw.trim() === "") {
    throw new ConfigError("ALLOWED_CHAT_ID is required");
  }
  const trimmed = raw.trim();
  if (!INTEGER.test(trimmed)) {
    throw new ConfigError("ALLOWED_CHAT_ID must be an integer");
  }
  const id = Number(trimmed);
  if (!Number.isSafeInteger(id) || String(id) !== canonicalInteger(trimmed)) {
    throw new ConfigError("ALLOWED_CHAT_ID is out of range");
  }
  return id;
}

function canonicalInteger(trimmed: string): string {
  const negative = trimmed.startsWith("-");
  const digits = (negative ? trimmed.slice(1) : trimmed).replace(/^0+/, "") || "0";
  if (negative && digits !== "0") return `-${digits}`;
  return digits;
}
