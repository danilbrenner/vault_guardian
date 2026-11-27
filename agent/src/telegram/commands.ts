// Command menu only. These are ordinary messages and go to the agent.
// Do not add bot.command handlers or a second dispatcher.

import type { BotCommand } from "grammy/types";

export const COMMANDS: readonly BotCommand[] = [
  { command: "report", description: "Full report focused on critical items" },
  { command: "sync", description: "Force MCP cache refresh" },
  { command: "status", description: "Show MCP cache status" },
  { command: "important", description: "Mark item as critical (by id or name)" },
  { command: "forget", description: "Remove item from critical" },
  { command: "start", description: "Start Vault Guardian" },
  { command: "help", description: "Show what Vault Guardian can do" },
];

export function registerCommands(bot: {
  api: { setMyCommands: (commands: readonly BotCommand[]) => Promise<true> };
}): Promise<true> {
  return bot.api.setMyCommands(COMMANDS);
}
