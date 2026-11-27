// One completed text generation per allowed message.
// The Vercel AI SDK and the OpenAI provider are the only model client.
// Do not call the OpenAI HTTP API, do not stream, and do not add another provider.

import { generateText, type LanguageModel } from "ai";
import { createOpenAI } from "@ai-sdk/openai";

export const HARDCODED_PROMPT =
  "You are Vault Guardian. Answer the user's message in plain text. Never ask for, accept, or repeat a password.";

export type ReplyModel = (apiKey: string, modelId: string) => LanguageModel;

export type ReplyRequest = {
  model: LanguageModel;
  prompt: string;
  instructions: string;
};

export type ReplyGenerate = (input: ReplyRequest) => Promise<{ text: string }>;

export function openAILanguageModel(apiKey: string, modelId: string): LanguageModel {
  return createOpenAI({ apiKey })(modelId);
}

export async function completeReply(
  message: string,
  options: {
    apiKey: string;
    model: string;
    generate?: ReplyGenerate;
    languageModel?: ReplyModel;
  },
): Promise<string> {
  const languageModel = options.languageModel ?? openAILanguageModel;
  const request: ReplyRequest = {
    model: languageModel(options.apiKey, options.model),
    prompt: message,
    instructions: HARDCODED_PROMPT,
  };
  const result = options.generate
    ? await options.generate(request)
    : await generateText(request);
  const text = result.text.trim();
  if (text === "") {
    throw new Error("model returned an empty reply");
  }
  return text;
}
