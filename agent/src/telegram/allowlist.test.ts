import assert from "node:assert/strict";
import { test } from "node:test";

import type { Bot, Context } from "grammy";
import type { BotFullInfo, Update } from "grammy/types";

import { ConfigError } from "../env.ts";
import { main } from "../index.ts";
import { allowOnly, type DropLog } from "./allowlist.ts";
import { createBot } from "./telegram.ts";

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

test("a missing chat id does not start polling", async () => {
  let started = false;
  await assert.rejects(
    () => main({ TELEGRAM_BOT_TOKEN: "123:abc" }, async () => {
      started = true;
    }),
    (err: unknown) => err instanceof ConfigError && /ALLOWED_CHAT_ID/.test(err.message),
  );
  assert.equal(started, false);
});

test("a foreign chat is dropped with no reply, before the next handler", async () => {
  const logged: number[] = [];
  let continued = false;
  await allowOnly(42, (chatId) => logged.push(chatId))(
    fakeChat(99),
    async () => {
      continued = true;
    },
  );
  assert.equal(continued, false);
  assert.deepEqual(logged, [99]);

  const { calls, lines } = await capture(textUpdate("do-not-log-this", 99), 42);
  assert.deepEqual(calls, []);
  assert.deepEqual(lines, [["dropped chat 99"]]);
  assert.equal(JSON.stringify(lines).includes("do-not-log-this"), false);
});

test("the allowed chat is answered and not logged", async () => {
  let continued = false;
  const logged: number[] = [];
  await allowOnly(42, (chatId) => logged.push(chatId))(
    fakeChat(42),
    async () => {
      continued = true;
    },
  );
  assert.equal(continued, true);
  assert.deepEqual(logged, []);

  const { calls, lines } = await capture(textUpdate("hello", 42), 42);
  assert.deepEqual(calls, [{ method: "sendMessage", chatId: 42, text: "Vault Guardian is listening." }]);
  assert.deepEqual(lines, []);
});

test("a group chat id is logged when it is not allowed", async () => {
  const { calls, lines } = await capture(textUpdate("hello", -200), -100);
  assert.deepEqual(calls, []);
  assert.deepEqual(lines, [["dropped chat -200"]]);
});

test("an update with no chat gets no reply and no log", async () => {
  const logged: number[] = [];
  const { calls } = await capture({ update_id: 1 }, 42, (chatId) => logged.push(chatId));
  assert.deepEqual(calls, []);
  assert.deepEqual(logged, []);
});

test("entrypoint uses ALLOWED_CHAT_ID and drops other chats", async () => {
  let bot: Bot | undefined;
  await main(
    { TELEGRAM_BOT_TOKEN: "123:from-env", ALLOWED_CHAT_ID: "-100" },
    async (started) => {
      bot = started;
    },
  );
  assert.ok(bot);
  const { calls, lines } = await capture(textUpdate("secret-text", 7), -100, undefined, bot);
  assert.deepEqual(calls, []);
  assert.deepEqual(lines, [["dropped chat 7"]]);
  assert.equal(JSON.stringify(lines).includes("secret-text"), false);
});

type Call = { method: string; chatId?: number; text?: string };

async function capture(
  update: Update,
  allowed: number,
  log?: DropLog,
  existing?: Bot,
): Promise<{ calls: Call[]; lines: unknown[][] }> {
  const lines: unknown[][] = [];
  const original = console.error;
  console.error = (...args: unknown[]) => {
    lines.push(args);
  };
  try {
    const bot = existing ?? createBot("123:test", { allowedChatId: allowed, log });
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
    return { calls, lines };
  } finally {
    console.error = original;
  }
}

function fakeChat(id: number): Context {
  return { chat: { id } } as Context;
}

function textUpdate(text: string, chatId: number): Update {
  return {
    update_id: 1,
    message: {
      message_id: 10,
      date: 1_700_000_000,
      chat: { id: chatId, type: "private", first_name: "Danil" },
      from: { id: chatId, is_bot: false, first_name: "Danil" },
      text,
    },
  };
}
