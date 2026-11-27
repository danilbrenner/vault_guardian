// Agent process entrypoint.
// Later slices start Telegram from here. This slice only loads the environment.

import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { loadEnvFile } from "./env.ts";

export function envFile(): string {
  return path.join(path.dirname(fileURLToPath(import.meta.url)), "..", ".env");
}

export function main(
  env: NodeJS.ProcessEnv = process.env,
  file = envFile(),
): void {
  loadEnvFile(file, env);
}

function isDirectRun(): boolean {
  const entry = process.argv[1];
  if (!entry) return false;
  return import.meta.url === pathToFileURL(path.resolve(entry)).href;
}

if (isDirectRun()) {
  main();
}
