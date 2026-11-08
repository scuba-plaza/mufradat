import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cardForRank } from "@/lib/lexicon";
import { POST } from "./route";
import { context } from "@/lib/server";

const river = { sentence: "شَرِبْتُ مِنْ مَاءِ النَّهْرِ", target: "مِنْ", translation: "I drank some of the river's water." };

function askFor(id: string) {
  return POST(new Request("http://127.0.0.1/api/mistral", { method: "POST", body: JSON.stringify({ id }) }));
}

describe("asking Mistral for an example", () => {
  let home: string;
  let answer: unknown;
  let requests: { agent_id: string; agent_version: number; inputs: string }[];

  beforeEach(() => {
    home = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-route-"));
    vi.stubEnv("XDG_CONFIG_HOME", path.join(home, "config"));
    vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home, "workspace"));
    vi.stubEnv("MISTRAL_API_KEY", "test-key");
    answer = river;
    requests = [];
    vi.stubGlobal("fetch", async (_url: string, init: { body: string }) => {
      requests.push(JSON.parse(init.body));
      return new Response(JSON.stringify({ outputs: [{ type: "message.output", content: JSON.stringify(answer) }] }));
    });
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("sends the card and no list of allowed words", async () => {
    const note = context().store.createNote(cardForRank(2000)!);
    await askFor(note.id);
    const user = requests[0].inputs;
    expect(user).not.toMatch(/vocabulary/i);
    expect(user.length).toBeLessThan(500);
  });

  it("asks the agent, in its version, for the example", async () => {
    const note = context().store.createNote(cardForRank(2)!);
    await askFor(note.id);
    expect(requests[0]).toMatchObject({ agent_id: "ag_01a1bac4790370849e18231695016789", agent_version: 4, store: false });
    for (const key of ["model", "messages", "temperature", "max_tokens", "top_p", "reasoning_effort", "response_format"]) {
      expect(requests[0]).not.toHaveProperty(key);
    }
  });

  it.each([
    { status: 401, problem: /rejected the API key/ },
    { status: 404, problem: /agent/ },
    { status: 429, problem: /rate limiting/ },
    { status: 503, problem: /server error/ },
  ])("keeps the failure for a $status on the card", async ({ status, problem }) => {
    vi.stubGlobal("fetch", async () => new Response("{}", { status }));
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.example).toBe("");
    expect(saved.exampleError).toMatch(problem);
  });

  it.each([
    { what: "no message output", outputs: [{ type: "tool.execution", content: "x" }] },
    { what: "no output at all", outputs: [] },
    { what: "an answer that is not JSON", outputs: [{ type: "message.output", content: "Here is a sentence." }] },
  ])("keeps the failure for $what", async ({ outputs }) => {
    vi.stubGlobal("fetch", async () => new Response(JSON.stringify({ outputs })));
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.exampleError).toMatch(/not the expected JSON/);
  });

  it("reads an answer that comes as chunks of text", async () => {
    vi.stubGlobal("fetch", async () =>
      new Response(JSON.stringify({ outputs: [{ type: "message.output", content: [{ type: "text", text: JSON.stringify(river).slice(0, 20) }, { type: "reference", text: "ignored" }, { type: "text", text: JSON.stringify(river).slice(20) }] }] })));
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.exampleEn).toBe(river.translation);
  });

  it("puts the brackets around the word Mistral names as the target", async () => {
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.example).toBe("شَرِبْتُ [مِنْ] مَاءِ النَّهْرِ");
    expect(saved.exampleEn).toBe(river.translation);
  });

  // The word of the card shows up in its sentence in another form than the card has it: attached to the next
  // word, with another case ending, with the article, or beside punctuation. The brackets still find it.
  it.each([
    { what: "a preposition attached to the next word", rank: 19, sentence: "كَتَبْتُ بِالْقَلَمِ فِي الْكِتَابِ", target: "بِ", marked: "كَتَبْتُ [بِ]الْقَلَمِ فِي الْكِتَابِ" },
    { what: "a particle that merges with the article", rank: 13, sentence: "ذَهَبْتُ لِلْمَدْرَسَةِ صَبَاحًا", target: "لِ", marked: "ذَهَبْتُ [لِ]لْمَدْرَسَةِ صَبَاحًا" },
    { what: "a particle named with a tatweel", rank: 19, sentence: "كَتَبْتُ بِالْقَلَمِ فِي الْكِتَابِ", target: "بِـ", marked: "كَتَبْتُ [بِ]الْقَلَمِ فِي الْكِتَابِ" },
    { what: "another case ending than the one named", rank: 115, sentence: "تَحَدَّثْتُ مَعَ أَبٍ كَرِيمٍ", target: "أَبٌ", marked: "تَحَدَّثْتُ مَعَ [أَبٍ] كَرِيمٍ" },
    { what: "a particle that the sentence carries a tatweel on", rank: 230, sentence: "هُوَ يَكْتُبُ كَـالْمُعَلِّمِ", target: "كَ", marked: "هُوَ يَكْتُبُ [كَ]الْمُعَلِّمِ" },
    { what: "the word with the article", rank: 2000, sentence: "الْكِتَابُ جَدِيدٌ جِدًّا", target: "كِتَابٌ", marked: "[الْكِتَابُ] جَدِيدٌ جِدًّا" },
    { what: "a word beside punctuation", rank: 2000, sentence: "هَلْ ذَهَبْتَ إِلَى الْمَدْرَسَةِ؟", target: "الْمَدْرَسَةِ", marked: "هَلْ ذَهَبْتَ إِلَى [الْمَدْرَسَةِ]؟" },
    { what: "a phrase of several words", rank: 2000, sentence: "هٰذِهِ دَارَةٌ إِلِكْتْرُونِيَّةٌ", target: "دَارَةٌ إِلِكْتْرُونِيَّةٌ", marked: "هٰذِهِ [دَارَةٌ إِلِكْتْرُونِيَّةٌ]" },
  ])("finds $what", async ({ rank, sentence, target, marked }) => {
    answer = { sentence, target, translation: "x" };
    const note = context().store.createNote(cardForRank(rank)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.example).toBe(marked);
    expect(saved.exampleError).toBeNull();
  });

  // When the word cannot be found for certain, nothing is guessed: the sentence is kept as it is, without brackets,
  // and the person marks the word.
  it.each([
    { what: "a target that is not in the sentence", sentence: "شَرِبْتُ مِنْ مَاءِ النَّهْرِ", target: "كِتَابٌ" },
    { what: "a target that is several words of the sentence", sentence: "كَانَ الْيَوْمُ كَانَ مُمْطِرًا", target: "كَانَ" },
    { what: "no target", sentence: "شَرِبْتُ مِنْ مَاءِ النَّهْرِ", target: "" },
    { what: "a target that would cut a word through its vowel mark", sentence: "ذَهَبْتُ مَعَ أَبِي إِلَى الْبَيْتِ", target: "أَب" },
  ])("keeps the sentence and leaves the word unmarked for $what", async ({ sentence, target }) => {
    answer = { sentence, target, translation: "The translation." };
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.example).toBe(sentence);
    expect(saved.exampleEn).toBe("The translation.");
    expect(saved.exampleError).toBeNull();
  });

  it("does not save a sentence that is not Arabic", async () => {
    answer = { sentence: "The book is new", target: "book", translation: "The book is new." };
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.example).toBe("");
    expect(saved.exampleError).toMatch(/no Arabic/);
  });
});
