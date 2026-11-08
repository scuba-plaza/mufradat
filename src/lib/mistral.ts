import { ApiError } from "./api-error";
import type { Config } from "./config";
import type { Note } from "./types";
import { POS_LABELS } from "./types";
import { autoMark, sentenceProblems, translationProblems } from "./validate";

const ENDPOINT = "https://api.mistral.ai/v1/conversations";
// The agent holds the model, the instructions and the sampling settings; the app sends only the card.
const AGENT_ID = "ag_01a1bac4790370849e18231695016789";
const AGENT_VERSION = 5;

function userPrompt(n: Note): string {
  const forms = n.forms.map((f) => `${f.label} ${f.arabic}`).join("; ");
  return [
    `Word: ${n.lemma}`,
    `Part of speech: ${POS_LABELS[n.pos]}`,
    `Meaning: ${n.english}`,
    n.root ? `Root: ${n.root}` : "",
    n.verbForm ? `Verb form: ${n.verbForm}` : "",
    forms ? `Forms: ${forms}` : "",
    n.hint ? `Hint: ${n.hint}` : "",
    "",
    "Write one example sentence for this word in this meaning.",
  ].filter((l, i, a) => l !== "" || a[i - 1] !== "").join("\n");
}

function httpProblem(status: number): string {
  if (status === 401) return "Mistral rejected the API key (401). Check mistral_api_key in config.toml.";
  if (status === 404) return "Mistral does not know the agent (404). Check the agent id and version in src/lib/mistral.ts.";
  if (status === 429) return "Mistral is rate limiting requests (429). Try again in a moment.";
  if (status >= 500) return `Mistral had a server error (${status}). Try again later.`;
  return `Mistral refused the request (${status}).`;
}

interface GeneratedExample {
  example: string;
  exampleEn: string;
}

/** One call to the Mistral agent. Throws ApiError with a message for the card; never retries. */
export async function generateExample(
  note: Note,
  cfg: Pick<Config, "mistralApiKey">,
  fetchImpl: typeof fetch = fetch,
): Promise<GeneratedExample> {
  if (!cfg.mistralApiKey) throw new ApiError("There is no Mistral API key. Set mistral_api_key in config.toml or MISTRAL_API_KEY.");
  let res: Response;
  try {
    res = await fetchImpl(ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${cfg.mistralApiKey}` },
      body: JSON.stringify({
        agent_id: AGENT_ID,
        agent_version: AGENT_VERSION,
        store: false,
        inputs: userPrompt(note),
      }),
    });
  } catch {
    throw new ApiError("Could not reach Mistral. Check the network connection and try again.");
  }
  if (!res.ok) throw new ApiError(httpProblem(res.status));
  let text: string;
  try {
    text = messageText(await res.json());
  } catch {
    throw new ApiError("Mistral's answer was not the expected JSON. Try again.");
  }
  // The agent thinks before it answers: when it runs out of tokens while thinking, the output has no text at all.
  if (text.trim() === "") throw new ApiError("Mistral's agent gave no answer; it may have run out of tokens while thinking. Try again.");
  let answer: unknown;
  try {
    answer = JSON.parse(text);
  } catch {
    throw new ApiError("Mistral's answer was not the expected JSON. Try again.");
  }
  const { sentence, translation } = answerParts(answer);
  const problems = [
    ...(sentence === "" ? ["The answer has no sentence."] : sentenceProblems(sentence)),
    ...(translation === "" ? ["The answer has no English translation."] : translationProblems(translation)),
  ];
  if (problems.length > 0) throw new ApiError(`Mistral's example was not saved. ${problems.join(" ")}`);
  // The agent does not name the word. The card's word is marked when it stands once in the sentence, and the
  // sentence is kept unmarked otherwise, for the person to mark.
  const words = [note.lemma, ...note.forms.map((f) => f.arabic)];
  return { example: autoMark(sentence, words) ?? sentence, exampleEn: translation };
}

/** The text of the answer: the content of the message output is a string, or a list of chunks of which the text ones make it. */
function messageText(body: unknown): string {
  const outputs = (body as { outputs?: { type?: string; content?: unknown }[] }).outputs;
  const content = outputs?.find((o) => o.type === "message.output")?.content;
  if (Array.isArray(content)) {
    return content.map((c: { type?: string; text?: string }) => (c.type === "text" ? (c.text ?? "") : "")).join("");
  }
  return typeof content === "string" ? content : "";
}

/** The agent answers with {meanings, sentence, translation}; the card keeps the sentence and its translation. */
function answerParts(answer: unknown): { sentence: string; translation: string } {
  const field = (key: string) => {
    const v = typeof answer === "object" && answer !== null ? (answer as Record<string, unknown>)[key] : undefined;
    return typeof v === "string" ? v.trim() : "";
  };
  return { sentence: field("sentence"), translation: field("translation") };
}
