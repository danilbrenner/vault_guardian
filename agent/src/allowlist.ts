// One allowed chat. Everyone else is dropped before a reply or a later agent call.

import type { Context, NextFunction } from "grammy";

import { ConfigError } from "./env.ts";

const INTEGER = /^-?\d+$/;

export type DropLog = (chatId: number) => void;

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

export function logDroppedChat(chatId: number): void {
  console.error(`dropped chat ${chatId}`);
}

export function allowOnly(
  allowed: number,
  log: DropLog = logDroppedChat,
): (ctx: Context, next: NextFunction) => Promise<void> {
  return async (ctx, next) => {
    const chatId = ctx.chat?.id;
    if (chatId === allowed) {
      await next();
      return;
    }
    if (chatId !== undefined) log(chatId);
  };
}

function canonicalInteger(trimmed: string): string {
  const negative = trimmed.startsWith("-");
  const digits = (negative ? trimmed.slice(1) : trimmed).replace(/^0+/, "") || "0";
  if (negative && digits !== "0") return `-${digits}`;
  return digits;
}
