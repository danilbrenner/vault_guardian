// One allowed chat. Everyone else is dropped before a reply or a later agent call.

import type { Context, NextFunction } from "grammy";

export type DropLog = (chatId: number) => void;

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
