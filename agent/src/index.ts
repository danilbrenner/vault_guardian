// Agent process entrypoint.
// Loads the environment, then long-polls Telegram. No command router.

import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import type { Bot } from "grammy";

import { loadEnvFile } from "./env.ts";
import { createBot, startPolling, telegramToken } from "./telegram.ts";

export function envFile(): string {
  return path.join(path.dirname(fileURLToPath(import.meta.url)), "..", ".env");
}

export async function main(
  env: NodeJS.ProcessEnv = process.env,
  file = envFile(),
  start: (bot: Bot) => Promise<void> = startPolling,
): Promise<void> {
  loadEnvFile(file, env);
  await start(createBot(telegramToken(env)));
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
