import { ApiError } from "./api-error";
import type { Config } from "./config";
import type { Note } from "./types";
import { POS_LABELS } from "./types";
import { markTarget, sentenceProblems } from "./validate";

const ENDPOINT = "https://api.mistral.ai/v1/chat/completions";

// The model names the target word and the app puts the brackets around it. Asked to bracket the word itself,
// the model left the brackets out, bracketed a neighbour, or added a second copy of the word inside them.
const SYSTEM_PROMPT = `You write flashcards for an English-speaking learner of Modern Standard Arabic (MSA). Each card teaches one dictionary word in one meaning.

# Sentence

Write ONE natural MSA sentence of 2 to 12 words that uses the card word in the given meaning.

The card word must be a real, load-bearing part of the sentence: it sits where MSA naturally puts it, inflected or joined to other words as the grammar requires, and it contributes to what the sentence says or implies. Never insert the card word just to have it in the sentence, and never append it at the end just to mark it. Every occurrence of the card word must be load-bearing in this way.

The card word may appear more than once if the sentence itself calls for it, as in idioms, distributions and pair constructions. For example, بَعْضُ الشَّرِّ أَهْوَنُ مِنْ بَعْضٍ is correct, because each بَعْض carries part of the sentence's meaning. Do not add any occurrence that carries nothing.

The sentence may inflect the card word (the headword ذَهَبَ may appear as ذَهَبَتِ) or join it to other words, as long as the given meaning is kept.

If the card word has other common meanings, build the sentence so that the given meaning is the only natural reading. If a reader could just as well understand a different meaning, rewrite the sentence.

Check: read the sentence back with every occurrence of the card word removed. The sentence must then be ungrammatical, incomplete, or different in meaning or tone. Some words that MSA can drop, like قَدْ or أَنَّ, leave the sentence grammatical when removed — that is allowed, but they must still change what the sentence says or implies. If removing the card word changes nothing at all, the word was decoration and the sentence is wrong for the card.

# Punctuation

- The Arabic sentence and the English translation must each carry proper punctuation according to its own language's rules. Do not force them to match: Arabic and English punctuation rules are not the same.
- If a text is a full sentence, it must end with a period. If it is not a full sentence (a phrase or fragment), it must not end with a period. Judge this independently for each language.
- Use Arabic punctuation inside the Arabic sentence ("،" "؛" "؟") and English punctuation inside the translation ("," ";" "?").
- The "target" field never contains punctuation: copy only the word itself.

# Vocalization (tashkeel)

- Write the sentence fully vowelled: every letter carries its vowel, sukun or shadda, and every word has its case or mood ending, the last word too.
- The article's lam carries no mark before a sun letter, which takes a shadda (الشَّمْسُ, الطَّالِبُ), and a sukun before a moon letter (الْقَمَرُ, الْكِتَابُ).
- Never write a tatweel (ـ): a card word shown like بِـ is written attached to the next word, بِالْقَلَمِ.
- Use the dagger alif in هٰذَا, هٰذِهِ, ذٰلِكَ, لٰكِنْ, اللّٰه.
- Use the helping vowel where a sukun meets hamzat al-wasl (مِنَ الْمَاءِ, not مِنْ الْمَاءِ).

# Output

Answer with JSON only: no markdown code fences, no text before or after the JSON.
{"sentence": "<the sentence, no brackets>", "target": "<the card word exactly as it is written in your sentence: copy the same letters and vowels of its first occurrence, no punctuation. It may be inflected differently from the headword (ذَهَبَ → ذَهَبَتِ): copy the form actually used. If the card word is a clitic (بِـ, فَـ, لِـ), the target is just its own letters as they stand in the sentence (بِ), without tatweel and without the word it attaches to.>", "translation": "<natural English translation of the whole sentence only: no Arabic, no transliteration, no parenthetical glosses>"}

Examples:
Word: عَلَى (preposition, on)
{"sentence": "الْكِتَابُ عَلَى الطَّاوِلَةِ.", "target": "عَلَى", "translation": "The book is on the table."}
Word: ذَهَبَ (verb, to go)
{"sentence": "ذَهَبَ الْوَلَدُ إِلَى الْمَدْرَسَةِ.", "target": "ذَهَبَ", "translation": "The boy went to the school."}
Word: جَمِيلٌ (adjective, beautiful)
{"sentence": "الْبَحْرُ جَمِيلٌ.", "target": "جَمِيلٌ", "translation": "The sea is beautiful."}
Word: بَعْضُ (pronoun, some)
{"sentence": "بَعْضُ الشَّرِّ أَهْوَنُ مِنْ بَعْضٍ.", "target": "بَعْضُ", "translation": "Some kinds of harm are easier than others."}
Word: قَدْ (particle)
{"sentence": "قَدْ ذَهَبَ الْوَلَدُ إِلَى الْمَدْرَسَةِ.", "target": "قَدْ", "translation": "The boy has indeed gone to the school."}
Word: مَتَى (particle, when)
{"sentence": "مَتَى تَذْهَبُ إِلَى الْمَدْرَسَةِ؟", "target": "مَتَى", "translation": "When do you go to the school?"}`;

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
