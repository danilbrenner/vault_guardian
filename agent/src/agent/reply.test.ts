import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import type { LanguageModel } from "ai";
import type { BotFullInfo, Update } from "grammy/types";

import { ConfigError, openaiApiKey, openaiModel } from "../env.ts";
import { main } from "../index.ts";
import type { DropLog } from "../telegram/allowlist.ts";
import { createBot } from "../telegram/telegram.ts";
import { HARDCODED_PROMPT, completeReply, type ReplyRequest } from "./reply.ts";

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

test("OPENAI_API_KEY and OPENAI_MODEL are required and trimmed", () => {
  assert.throws(() => openaiApiKey({}), ConfigError);
  assert.throws(() => openaiApiKey({ OPENAI_API_KEY: "   " }), ConfigError);
  assert.throws(() => openaiModel({ OPENAI_API_KEY: "sk-test" }), ConfigError);
  assert.throws(() => openaiModel({ OPENAI_MODEL: "  " }), ConfigError);
  assert.equal(openaiApiKey({ OPENAI_API_KEY: " sk-test " }), "sk-test");
  assert.equal(openaiModel({ OPENAI_MODEL: " gpt-test " }), "gpt-test");
});

test("a missing model config does not start polling", async () => {
  let started = false;
  await assert.rejects(
    () => main({
      TELEGRAM_BOT_TOKEN: "123:abc",
      ALLOWED_CHAT_ID: "42",
      OPENAI_API_KEY: "sk-test",
    }, async () => {
      started = true;
    }),
    (err: unknown) => err instanceof ConfigError && /OPENAI_MODEL/.test(err.message),
  );
  assert.equal(started, false);

  started = false;
  await assert.rejects(
    () => main({
      TELEGRAM_BOT_TOKEN: "123:abc",
      ALLOWED_CHAT_ID: "42",
    }, async () => {
      started = true;
    }),
    (err: unknown) => err instanceof ConfigError && /OPENAI_API_KEY/.test(err.message),
  );
  assert.equal(started, false);
});

test("an allowed message is one completed generation and the reply is sent to that chat", async () => {
  const requests: ReplyRequest[] = [];
  const models: Array<{ apiKey: string; model: string }> = [];
  const reply = (text: string) => completeReply(text, {
    apiKey: "sk-test",
    model: "gpt-test",
    languageModel: (apiKey, model) => {
      models.push({ apiKey, model });
      return { provider: "openai", modelId: model } as LanguageModel;
    },
    generate: async (input) => {
      requests.push(input);
      return { text: "  GitHub is weak.  " };
    },
  });

  const { calls } = await deliver(textUpdate("what should I change?", 42), 42, reply);

  assert.equal(requests.length, 1);
  assert.equal(requests[0]?.prompt, "what should I change?");
  assert.equal(requests[0]?.instructions, HARDCODED_PROMPT);
  assert.deepEqual(models, [{ apiKey: "sk-test", model: "gpt-test" }]);
  assert.equal(JSON.stringify(requests).includes("sk-test"), false);
  assert.deepEqual(calls, [{ method: "sendMessage", chatId: 42, text: "GitHub is weak." }]);
});

test("a dropped chat does not generate a reply", async () => {
  let generated = 0;
  const reply = async () => {
    generated += 1;
    return "should-not-send";
  };
  const { calls } = await deliver(textUpdate("secret-text", 99), 42, reply, () => {});
  assert.equal(generated, 0);
  assert.deepEqual(calls, []);
});

test("an empty completion is not sent", async () => {
  await assert.rejects(
    () => completeReply("hello", {
      apiKey: "sk-test",
      model: "gpt-test",
      languageModel: () => ({}) as LanguageModel,
      generate: async () => ({ text: "   " }),
    }),
    /empty reply/,
  );
});

test("the prompt does not request a password and there is no password field", () => {
  assert.match(HARDCODED_PROMPT, /Never ask for, accept, or repeat a password/);
  assert.equal(/\bpassword\s*:/.test(HARDCODED_PROMPT), false);
  const src = source("reply.ts");
  assert.equal(/\bpassword\s*:/.test(src), false);
  assert.equal(src.includes("streamText"), false);
  assert.equal(src.includes("api.openai.com"), false);
  assert.equal(/\bfetch\s*\(/.test(src), false);
});

test("model calls go through the AI SDK and the OpenAI provider", () => {
  const src = source("reply.ts");
  assert.match(src, /from "ai"/);
  assert.match(src, /generateText/);
  assert.match(src, /@ai-sdk\/openai/);
  assert.match(src, /createOpenAI/);
  assert.equal(src.includes("@ai-sdk/anthropic"), false);
  assert.equal(src.includes("streamText"), false);

  const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");
  for (const file of walk(root)) {
    if (file.endsWith(".test.ts")) continue;
    const text = readFileSync(file, "utf8");
    assert.equal(text.includes("api.openai.com"), false, file);
    assert.equal(text.includes("streamText"), false, file);
  }
});

type Call = { method: string; chatId?: number; text?: string };

async function deliver(
  update: Update,
  allowed: number,
  reply: (text: string) => Promise<string>,
  log?: DropLog,
): Promise<{ calls: Call[] }> {
  const bot = createBot("123:test", { allowedChatId: allowed, reply, log });
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

function source(name: string): string {
  return readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), name), "utf8");
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
