// Agent process entrypoint.
// Reads the process environment, then long-polls Telegram for one allowed chat.

import path from "node:path";
import { pathToFileURL } from "node:url";

import type { Bot } from "grammy";

import { allowedChatId, telegramToken } from "./env.ts";
import { createBot, startPolling } from "./telegram/telegram.ts";

export async function main(
  env: NodeJS.ProcessEnv = process.env,
  start: (bot: Bot) => Promise<void> = startPolling,
): Promise<void> {
  const token = telegramToken(env);
  const chatId = allowedChatId(env);
  await start(createBot(token, { allowedChatId: chatId }));
}

function isDirectRun(): boolean {
  const entry = process.argv[1];
  if (!entry) return false;
  return import.meta.url === pathToFileURL(path.resolve(entry)).href;
}

if (isDirectRun()) {
  main().catch((err: unknown) => {
    const message = err instanceof Error ? err.message : String(err);
    console.error(`agent: ${message}`);
    process.exitCode = 1;
  });
}
