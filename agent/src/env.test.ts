import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { loadEnvFile } from "./env.ts";

test("loads assignments and ignores comments and blank lines", () => {
  const env: NodeJS.ProcessEnv = {};
  withFile("\n# comment\r\nFOO=bar\r\nexport BAZ=qux\nEMPTY=\n", (file) => {
    loadEnvFile(file, env);
  });
  assert.equal(env.FOO, "bar");
  assert.equal(env.BAZ, "qux");
  assert.equal(env.EMPTY, "");
});

test("strips quotes and keeps equals signs in the value", () => {
  const env: NodeJS.ProcessEnv = {};
  withFile("QUOTED=\"a b\"\nSINGLE='c'\nPAIR=a=b\n", (file) => {
    loadEnvFile(file, env);
  });
  assert.equal(env.QUOTED, "a b");
  assert.equal(env.SINGLE, "c");
  assert.equal(env.PAIR, "a=b");
});

test("does not override variables already in the environment", () => {
  const env: NodeJS.ProcessEnv = { KEEP: "from-env", EMPTY: "" };
  withFile("KEEP=from-file\nEMPTY=filled\nNEW=yes\n", (file) => {
    loadEnvFile(file, env);
  });
  assert.equal(env.KEEP, "from-env");
  assert.equal(env.EMPTY, "");
  assert.equal(env.NEW, "yes");
});

test("a missing env file is not an error", () => {
  const env: NodeJS.ProcessEnv = {};
  loadEnvFile(path.join(tmpdir(), "vault-guardian-missing.env"), env);
  assert.deepEqual(env, {});
});

test("runtime dependency is grammY only", () => {
  const pkgPath = path.join(
    path.dirname(fileURLToPath(import.meta.url)),
    "..",
    "package.json",
  );
  const pkg = JSON.parse(readFileSync(pkgPath, "utf8")) as {
    dependencies?: Record<string, string>;
  };
  const deps: Record<string, string> = pkg.dependencies ?? {};
  assert.deepEqual(Object.keys(deps), ["grammy"]);
});

function withFile(contents: string, run: (file: string) => void): void {
  const dir = mkdtempSync(path.join(tmpdir(), "vg-env-"));
  const file = path.join(dir, ".env");
  writeFileSync(file, contents);
  try {
    run(file);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}
