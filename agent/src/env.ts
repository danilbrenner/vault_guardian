// Environment loading for the agent process.
// A missing file is fine: Docker injects variables and local runs may use the shell.
// Existing variables win, so a file cannot override the process environment.

import { readFileSync } from "node:fs";

const assignment =
  /^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/;

export function loadEnvFile(file: string, env: NodeJS.ProcessEnv): void {
  let text: string;
  try {
    text = readFileSync(file, "utf8");
  } catch (err) {
    if (isMissing(err)) return;
    throw err;
  }
  for (const [key, value] of parseEnv(text)) {
    if (env[key] === undefined) env[key] = value;
  }
}

function parseEnv(text: string): Array<[string, string]> {
  const pairs: Array<[string, string]> = [];
  const body = text.charCodeAt(0) === 0xfeff ? text.slice(1) : text;
  for (const rawLine of body.split("\n")) {
    const line = rawLine.replace(/\r$/, "").trim();
    if (line === "" || line.startsWith("#")) continue;
    const match = assignment.exec(line);
    if (!match) continue;
    pairs.push([match[1] ?? "", unquote(match[2] ?? "")]);
  }
  return pairs;
}

function unquote(raw: string): string {
  const value = raw.trim();
  if (value.length >= 2) {
    const open = value[0];
    const close = value[value.length - 1];
    if ((open === '"' && close === '"') || (open === "'" && close === "'")) {
      return value.slice(1, -1);
    }
  }
  return value;
}

function isMissing(err: unknown): boolean {
  return (
    typeof err === "object" &&
    err !== null &&
    "code" in err &&
    err.code === "ENOENT"
  );
}
