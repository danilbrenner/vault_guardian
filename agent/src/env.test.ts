import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { allowedChatId, ConfigError, openaiApiKey, openaiModel, telegramToken } from "./env.ts";

test("TELEGRAM_BOT_TOKEN is required and trimmed", () => {
  assert.throws(() => telegramToken({}), ConfigError);
  assert.throws(() => telegramToken({ TELEGRAM_BOT_TOKEN: "   " }), ConfigError);
  assert.equal(telegramToken({ TELEGRAM_BOT_TOKEN: " 123:from-env " }), "123:from-env");
});

test("ALLOWED_CHAT_ID is required to start", () => {
  assert.throws(() => allowedChatId({}), ConfigError);
  assert.throws(() => allowedChatId({ ALLOWED_CHAT_ID: "" }), ConfigError);
  assert.throws(() => allowedChatId({ ALLOWED_CHAT_ID: "   " }), ConfigError);
  assert.equal(allowedChatId({ ALLOWED_CHAT_ID: " 42 " }), 42);
  assert.equal(allowedChatId({ ALLOWED_CHAT_ID: "-100" }), -100);
});

test("a non-integer or out-of-range id fails closed", () => {
  assert.throws(() => allowedChatId({ ALLOWED_CHAT_ID: "12abc" }), ConfigError);
  assert.throws(() => allowedChatId({ ALLOWED_CHAT_ID: "1.5" }), ConfigError);
  assert.throws(
    () => allowedChatId({ ALLOWED_CHAT_ID: "9007199254740993" }),
    ConfigError,
  );
});

test("runtime dependencies are grammY, the AI SDK, and the OpenAI provider", () => {
  const pkgPath = path.join(
    path.dirname(fileURLToPath(import.meta.url)),
    "..",
    "package.json",
  );
  const pkg = JSON.parse(readFileSync(pkgPath, "utf8")) as {
    dependencies?: Record<string, string>;
    devDependencies?: Record<string, string>;
  };
  const deps: Record<string, string> = pkg.dependencies ?? {};
  const devDeps: Record<string, string> = pkg.devDependencies ?? {};
  assert.deepEqual(Object.keys(deps).sort(), ["@ai-sdk/openai", "ai", "grammy"]);
  assert.equal(typeof devDeps["dotenv"], "string");
  assert.equal(devDeps["dotenv-cli"], undefined);
  for (const name of [...Object.keys(deps), ...Object.keys(devDeps)]) {
    assert.equal(/telegraf|node-telegram-bot-api|anthropic|langchain/i.test(name), false, name);
  }
  assert.equal(openaiApiKey({ OPENAI_API_KEY: "sk-test" }), "sk-test");
  assert.equal(openaiModel({ OPENAI_MODEL: "gpt-test" }), "gpt-test");
});
