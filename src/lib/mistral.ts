import { ApiError } from "./api-error";
import type { Config } from "./config";
import type { Note } from "./types";
import { POS_LABELS } from "./types";
import { markTarget, sentenceProblems } from "./validate";

const ENDPOINT = "https://api.mistral.ai/v1/conversations";
// The agent holds the model, the instructions and the sampling settings; the app sends only the card.
const AGENT_ID = "ag_01a1bac4790370849e18231695016789";
const AGENT_VERSION = 4;

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
  let parsed: { sentence?: unknown; target?: unknown; translation?: unknown };
  try {
    const body = (await res.json()) as { outputs?: { type?: string; content?: unknown }[] };
    const content = body.outputs?.find((o) => o.type === "message.output")?.content;
    // The content is a string, or a list of chunks of which the text ones make the answer.
    const answer = Array.isArray(content)
      ? content.map((c: { type?: string; text?: string }) => (c.type === "text" ? (c.text ?? "") : "")).join("")
      : content;
    parsed = JSON.parse(typeof answer === "string" ? answer : "");
  } catch {
    throw new ApiError("Mistral's answer was not the expected JSON. Try again.");
  }
  const text = (v: unknown) => (typeof v === "string" ? v.trim() : "");
  // No brackets of its own, and no tatweel: the card shows a particle like كَـ with one, but a sentence never does.
  const sentence = text(parsed.sentence).replace(/[[\]ـ]/g, "");
  const exampleEn = text(parsed.translation);
  const problems = sentence === "" ? ["The answer has no sentence."] : sentenceProblems(sentence);
  if (exampleEn === "") problems.push("The answer has no English translation.");
  if (problems.length > 0) throw new ApiError(`Mistral's example was not saved. ${problems.join(" ")}`);
  // A sentence whose word cannot be found for certain is kept without brackets, for the person to mark.
  return { example: markTarget(sentence, text(parsed.target)) ?? sentence, exampleEn };
}
