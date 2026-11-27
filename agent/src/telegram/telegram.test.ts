import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { Bot } from "grammy";
import type { BotFullInfo, Update } from "grammy/types";

import { ConfigError } from "../env.ts";
import { main } from "../index.ts";
import { createBot, startPolling, type Reply } from "./telegram.ts";

const REPLY = "Vault Guardian is listening.";

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

test("a text update is received and answered with one reply", { timeout: 2_000 }, async () => {
  const { calls } = await deliver(textUpdate("hello", 42));
  assert.deepEqual(calls, [{ method: "sendMessage", chatId: 42, text: REPLY }]);
});

test("a command is an ordinary message, not a separate handler", { timeout: 2_000 }, async () => {
  const { calls } = await deliver(textUpdate("/start", 7));
  assert.deepEqual(calls, [{ method: "sendMessage", chatId: 7, text: REPLY }]);
});

test("startPolling uses long polling and does not pass webhook options", async () => {
  const args: unknown[][] = [];
  await startPolling({
    api: {
      setMyCommands: async () => true,
    },
    start: async (...received: unknown[]) => {
      args.push(received);
    },
  });
  assert.deepEqual(args, [[]]);
});

test("a missing token fails closed and does not start polling", async () => {
  let started = false;
  await assert.rejects(
    () => main({}, async () => {
      started = true;
    }),
    (err: unknown) => err instanceof ConfigError && /TELEGRAM_BOT_TOKEN/.test(err.message),
  );
  assert.equal(started, false);
});

test("entrypoint uses the token from the environment and starts polling", async () => {
  let token = "";
  await main(
    {
      TELEGRAM_BOT_TOKEN: " 123:from-env ",
      ALLOWED_CHAT_ID: "42",
      OPENAI_API_KEY: "sk-test",
      OPENAI_MODEL: "gpt-test",
    },
    async (bot) => {
      token = bot.token;
    },
  );
  assert.equal(token, "123:from-env");
});

test("source does not call the Telegram HTTP API", () => {
  const dir = path.dirname(fileURLToPath(import.meta.url));
  for (const name of readdirSync(dir)) {
    if (!name.endsWith(".ts") || name.endsWith(".test.ts")) continue;
    const text = readFileSync(path.join(dir, name), "utf8");
    assert.equal(text.includes("api.telegram.org"), false, name);
    assert.equal(/\bfetch\s*\(/.test(text), false, name);
    assert.equal(text.includes("readFileSync"), false, name);
  }
});

type Call = { method: string; chatId?: number; text?: string };

async function deliver(
  update: Update,
  reply: Reply = async () => REPLY,
): Promise<{ calls: Call[] }> {
  const chatId = update.message?.chat.id;
  if (chatId === undefined) throw new Error("test update has no chat");
  const bot = createBot("123:test", { allowedChatId: chatId, reply });
  bot.botInfo = botInfo;
  const calls: Call[] = [];
  bot.api.config.use(async (_prev, method, payload) => {
    const chatId = "chat_id" in payload ? Number(payload.chat_id) : undefined;
    const text = "text" in payload ? String(payload.text) : undefined;
    calls.push({ method, chatId, text });
    const result = method === "sendMessage"
      ? {
          message_id: 2,
          date: 1,
          chat: { id: chatId ?? 0, type: "private" as const, first_name: "Danil" },
          text: text ?? "",
        }
      : true;
    return { ok: true, result } as Awaited<ReturnType<typeof _prev>>;
  });
  await bot.handleUpdate(update);
  return { calls };
}

function textUpdate(text: string, chatId: number): Update {
  const command = text.startsWith("/")
    ? {
        entities: [
          {
            type: "bot_command" as const,
            offset: 0,
            length: text.split(" ")[0]?.length ?? text.length,
          },
        ],
      }
    : {};
  return {
    update_id: 1,
    message: {
      message_id: 10,
      date: 1_700_000_000,
      chat: { id: chatId, type: "private", first_name: "Danil" },
      from: { id: chatId, is_bot: false, first_name: "Danil" },
      text,
      ...command,
    },
  };
}
