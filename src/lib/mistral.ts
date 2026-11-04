import { ApiError } from "./api-error";
import type { Config } from "./config";
import type { Note } from "./types";
import { POS_LABELS } from "./types";
import { markTarget, sentenceProblems } from "./validate";

const ENDPOINT = "https://api.mistral.ai/v1/chat/completions";

// The model names the target word and the app puts the brackets around it. Asked to bracket the word itself,
// the model left the brackets out, bracketed a neighbour, or added a second copy of the word inside them.
const SYSTEM_PROMPT = `You write flashcards for an English-speaking learner of Modern Standard Arabic (MSA). Each card teaches one dictionary word in one meaning.

Write ONE natural MSA sentence of 4 to 8 words that uses the card word in the given meaning.

The card word must be a real part of the sentence: it appears exactly once, where the grammar needs it, inflected or joined to other words as MSA requires. Never repeat the card word and never add it at the end just to mark it. Read the sentence back without it: it must then be incomplete.

Write the sentence fully vowelled (tashkeel): every letter carries its vowel, sukun or shadda, and every word has its case or mood ending, the last word too. The article's lam carries no mark before a sun letter, which takes a shadda (الشَّمْسُ, الطَّالِبُ), and a sukun before a moon letter (الْقَمَرُ, الْكِتَابُ). Never write a tatweel (ـ): a card word shown like بِـ is written attached to the next word, بِالْقَلَمِ. Use the dagger alif in هٰذَا, هٰذِهِ, ذٰلِكَ, لٰكِنْ, اللّٰه. Use the helping vowel where a sukun meets hamzat al-wasl (مِنَ الْمَاءِ, not مِنْ الْمَاءِ).

Answer with JSON only:
{"sentence": "<the sentence, no brackets>", "target": "<the card word exactly as it is written in your sentence: copy the same letters and vowels>", "translation": "<natural English translation of the whole sentence>"}

Examples:
Word: عَلَى (preposition, on)
{"sentence": "الْكِتَابُ عَلَى الطَّاوِلَةِ", "target": "عَلَى", "translation": "The book is on the table."}
Word: ذَهَبَ (verb, to go)
{"sentence": "ذَهَبَ الْوَلَدُ إِلَى الْمَدْرَسَةِ", "target": "ذَهَبَ", "translation": "The boy went to the school."}`;

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
  if (status === 404) return "Mistral does not know the model (404). Check mistral_model in config.toml.";
  if (status === 429) return "Mistral is rate limiting requests (429). Try again in a moment.";
  if (status >= 500) return `Mistral had a server error (${status}). Try again later.`;
  return `Mistral refused the request (${status}).`;
}

interface GeneratedExample {
  example: string;
  exampleEn: string;
}

/** One call to Mistral. Throws ApiError with a message for the card; never retries. */
export async function generateExample(
  note: Note,
  cfg: Pick<Config, "mistralApiKey" | "mistralModel" | "mistralReasoningEffort">,
  fetchImpl: typeof fetch = fetch,
): Promise<GeneratedExample> {
  if (!cfg.mistralApiKey) throw new ApiError("There is no Mistral API key. Set mistral_api_key in config.toml or MISTRAL_API_KEY.");
  let res: Response;
  try {
    res = await fetchImpl(ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${cfg.mistralApiKey}` },
      body: JSON.stringify({
        model: cfg.mistralModel,
        temperature: 0.3,
        ...(cfg.mistralReasoningEffort ? { reasoning_effort: cfg.mistralReasoningEffort } : {}),
        response_format: { type: "json_object" },
        messages: [
          { role: "system", content: SYSTEM_PROMPT },
          { role: "user", content: userPrompt(note) },
        ],
      }),
    });
  } catch {
    throw new ApiError("Could not reach Mistral. Check the network connection and try again.");
  }
  if (!res.ok) throw new ApiError(httpProblem(res.status));
  let parsed: { sentence?: unknown; target?: unknown; translation?: unknown };
  try {
    const body = (await res.json()) as { choices?: { message?: { content?: string } }[] };
    parsed = JSON.parse(body.choices?.[0]?.message?.content ?? "");
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
