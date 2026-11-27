import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import type { BotCommand, BotFullInfo, Update } from "grammy/types";

import { createBot, startPolling, type Reply } from "./telegram.ts";
import { COMMANDS, registerCommands } from "./commands.ts";

const botInfo: BotFullInfo = {
  id: 1,
  is_bot: true,
  first_name: "Vault Guardian",
  username: "vault_guardian_bot",
  can_join_groups: false,
  can_read_all_group_messages: false,
  supports_guest_queries: false,
  supports_inline_queries: false,
  needs_inline_query_location: false,
  can_connect_to_business: false,
  has_main_web_app: false,
  has_topics_enabled: false,
  allows_users_to_create_topics: false,
  has_topic_sidebar: false,
  can_manage_bots: false,
  supports_join_request_queries: false,
};

test("the command menu is the spec list plus start and help", async () => {
  const published: BotCommand[][] = [];
  await registerCommands({
    api: {
      setMyCommands: async (commands) => {
        published.push([...commands]);
        return true;
      },
    },
  });
  assert.deepEqual(published, [COMMANDS]);
  assert.deepEqual(COMMANDS.map((command) => command.command), [
    "report",
    "sync",
    "status",
    "important",
    "forget",
    "start",
    "help",
  ]);
});

test("commands are registered before long polling and are not webhook options", async () => {
  const order: string[] = [];
  let published: readonly BotCommand[] = [];
  const startArgs: unknown[][] = [];
  await startPolling({
    api: {
      setMyCommands: async (commands) => {
        order.push("register");
        published = commands;
        return true;
      },
    },
    start: async (...received: unknown[]) => {
      order.push("start");
      startArgs.push(received);
    },
  });
  assert.deepEqual(order, ["register", "start"]);
  assert.deepEqual(startArgs, [[]]);
  assert.equal(published, COMMANDS);
});

test("polling does not start when command registration fails", async () => {
  let started = false;
  await assert.rejects(
    () => startPolling({
      api: {
        setMyCommands: async () => {
          throw new Error("menu failed");
        },
      },
      start: async () => {
        started = true;
      },
    }),
    /menu failed/,
  );
  assert.equal(started, false);
});

test("a command is an ordinary message sent to the agent", async () => {
  const seen: string[] = [];
  const reply: Reply = async (text) => {
    seen.push(text);
    return `heard ${text}`;
  };
  for (const text of ["/report", "/sync", "/status", "/important GitHub", "/forget github", "/start", "/help"]) {
    const { calls } = await deliver(textUpdate(text, 42), reply);
    assert.deepEqual(calls, [{ method: "sendMessage", chatId: 42, text: `heard ${text}` }]);
  }
  assert.deepEqual(seen, [
    "/report",
    "/sync",
    "/status",
    "/important GitHub",
    "/forget github",
    "/start",
    "/help",
  ]);
});

test("there is no command dispatcher or stub handler", () => {
  const dir = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");
  for (const file of walk(dir)) {
    if (file.endsWith(".test.ts")) continue;
    const text = readFileSync(file, "utf8");
    assert.equal(/\.command\s*\(/.test(text), false, file);
    assert.equal(text.includes("bot.hears"), false, file);
    assert.equal(text.includes("stub handler"), false, file);
  }
});

type Call = { method: string; chatId?: number; text?: string };

async function deliver(update: Update, reply: Reply): Promise<{ calls: Call[] }> {
  const bot = createBot("123:test", { allowedChatId: 42, reply });
  bot.botInfo = botInfo;
  const calls: Call[] = [];
  bot.api.config.use(async (_prev, method, payload) => {
    const chatId = "chat_id" in payload ? Number(payload.chat_id) : undefined;
    const text = "text" in payload ? String(payload.text) : undefined;
    calls.push({ method, chatId, text });
    return {
      ok: true,
      result: {
        message_id: 2,
        date: 1,
        chat: { id: chatId ?? 0, type: "private" as const, first_name: "Danil" },
        text: text ?? "",
      },
    } as Awaited<ReturnType<typeof _prev>>;
  });
  await bot.handleUpdate(update);
  return { calls };
}

function textUpdate(text: string, chatId: number): Update {
  const command = text.split(" ")[0] ?? text;
  return {
    update_id: 1,
    message: {
      message_id: 10,
      date: 1_700_000_000,
      chat: { id: chatId, type: "private", first_name: "Danil" },
      from: { id: chatId, is_bot: false, first_name: "Danil" },
      text,
      entities: [{ type: "bot_command", offset: 0, length: command.length }],
    },
  };
}

function walk(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) out.push(...walk(full));
    else if (entry.name.endsWith(".ts")) out.push(full);
  }
  return out;
}
