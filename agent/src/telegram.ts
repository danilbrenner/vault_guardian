// Telegram client. grammY long polling only.
// Do not call the Telegram HTTP API from this process, and do not add another client.

import { Bot } from "grammy";

import { allowOnly, type DropLog } from "./allowlist.ts";
import { ConfigError } from "./env.ts";

const REPLY = "Vault Guardian is listening.";

export function telegramToken(env: NodeJS.ProcessEnv): string {
  const raw = env.TELEGRAM_BOT_TOKEN;
  if (raw === undefined || raw.trim() === "") {
    throw new ConfigError("TELEGRAM_BOT_TOKEN is required");
  }
  return raw.trim();
}

export function createBot(
  token: string,
  options: { allowedChatId: number; log?: DropLog },
): Bot {
  const bot = new Bot(token);
  bot.use(allowOnly(options.allowedChatId, options.log));
  bot.on("message:text", async (ctx) => {
    await ctx.reply(REPLY);
  });
  return bot;
}

export function startPolling(bot: {
  start: () => Promise<void>;
}): Promise<void> {
  return bot.start();
}
