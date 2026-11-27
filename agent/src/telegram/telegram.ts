// Telegram client. grammY long polling only.
// Do not call the Telegram HTTP API from this process, and do not add another client.

import { Bot } from "grammy";
import type { BotCommand } from "grammy/types";

import { allowOnly, type DropLog } from "./allowlist.ts";
import { registerCommands } from "./commands.ts";

export type Reply = (text: string) => Promise<string>;

export function createBot(
  token: string,
  options: { allowedChatId: number; log?: DropLog; reply: Reply },
): Bot {
  const bot = new Bot(token);
  bot.use(allowOnly(options.allowedChatId, options.log));
  bot.on("message:text", async (ctx) => {
    await ctx.react("👀");
    const reply = await options.reply(ctx.message.text);
    await ctx.reply(reply, {
      reply_parameters: {
        message_id: ctx.message.message_id,
        quote: ctx.message.text,
      },
    });
    await ctx.react("👍");
  });
  return bot;
}

export function startPolling(bot: {
  api: { setMyCommands: (commands: readonly BotCommand[]) => Promise<true> };
  start: () => Promise<void>;
}): Promise<void> {
  return registerCommands(bot).then(() => bot.start());
}
