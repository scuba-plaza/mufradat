import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cardForRank } from "@/lib/lexicon";
import { POST } from "./route";
import { context } from "@/lib/server";

const river = {
  meanings: ["from", "of"],
  sentence: "شَرِبَ الْوَلَدُ مِنْ مَاءِ النَّهْرِ الْبَارِدِ",
  translation: "The boy drank from the cold river water.",
};

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
    expect(requests[0]).toMatchObject({ agent_id: "ag_01a1bac4790370849e18231695016789", agent_version: 5, store: false });
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
    { what: "no message output", outputs: [{ type: "tool.execution", content: "x" }], problem: /gave no answer/ },
    { what: "no output at all", outputs: [], problem: /gave no answer/ },
    { what: "an answer that is not JSON", outputs: [{ type: "message.output", content: "Here is a sentence." }], problem: /not the expected JSON/ },
  ])("keeps the failure for $what", async ({ outputs, problem }) => {
    vi.stubGlobal("fetch", async () => new Response(JSON.stringify({ outputs })));
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.exampleError).toMatch(problem);
  });

  it("reads an answer that comes as chunks of text", async () => {
    vi.stubGlobal("fetch", async () =>
      new Response(JSON.stringify({ outputs: [{ type: "message.output", content: [{ type: "text", text: JSON.stringify(river).slice(0, 20) }, { type: "reference", text: "ignored" }, { type: "text", text: JSON.stringify(river).slice(20) }] }] })));
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.exampleEn).toBe(river.translation);
  });

  it("saves the sentence with the card's word marked, and the translation", async () => {
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.example).toBe("شَرِبَ الْوَلَدُ [مِنْ] مَاءِ النَّهْرِ الْبَارِدِ");
    expect(saved.exampleEn).toBe(river.translation);
    expect(saved.exampleError).toBeNull();
  });

  it("marks a form of the card's word when the headword is not in the sentence", async () => {
    answer = { ...river, sentence: "ذَكَرَ الْمُؤَرِّخُ آبَاءَ الثَّوْرَةِ فِي كِتَابِهِ" };
    const note = context().store.createNote(cardForRank(115)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.example).toBe("ذَكَرَ الْمُؤَرِّخُ [آبَاءَ] الثَّوْرَةِ فِي كِتَابِهِ");
  });

  it("reads the answer after the agent's thinking chunk", async () => {
    vi.stubGlobal("fetch", async () =>
      new Response(JSON.stringify({ outputs: [{ type: "message.output", content: [
        { type: "thinking", thinking: [{ type: "text", text: "hmm" }] },
        { type: "text", text: JSON.stringify(river) },
      ] }] })));
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.exampleEn).toBe(river.translation);
  });

  it("keeps the sentence unmarked when the card's word is not in it", async () => {
    answer = { ...river, sentence: "ذَهَبَ الْوَلَدُ إِلَى الْمَدْرَسَةِ الْقَرِيبَةِ صَبَاحًا" };
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.example).toBe((answer as { sentence: string }).sentence);
    expect(saved.exampleError).toBeNull();
  });

  it("keeps the sentence unmarked when the card's word stands twice in it", async () => {
    answer = { ...river, sentence: "أَخَذَ الْوَلَدُ مِنْ الْبَيْتِ ثُمَّ مِنْ الْمَدْرَسَةِ" };
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.example).toBe((answer as { sentence: string }).sentence);
  });

  // Each rule of the agent's system prompt that the code can check: 2 to 12 words, Arabic, every word vocalized,
  // and an English translation.
  it.each([
    { what: "too few words", sentence: "شَرِبَ", problem: /1 word; it must have 2 to 12/ },
    { what: "too many words", sentence: "شَرِبَ الْوَلَدُ مِنْ مَاءِ النَّهْرِ الْبَارِدِ فِي الصَّبَاحِ الْبَاكِرِ قَبْلَ الذَّهَابِ إِلَى الْمَدْرَسَةِ", problem: /13 words; it must have 2 to 12/ },
    { what: "a word without vowels", sentence: "شَرِبَ الولد مِنْ مَاءِ النَّهْرِ الْبَارِدِ", problem: /without a vowel mark: الولد\./ },
    { what: "a word with only a shadda", sentence: "شَرِبَ الشّمس مِنْ مَاءِ النَّهْرِ الْبَارِدِ", problem: /without a vowel mark: الشّمس\./ },
    { what: "square brackets", sentence: "شَرِبَ [الْوَلَدُ] مِنْ مَاءِ النَّهْرِ الْبَارِدِ", problem: /square brackets/ },
    { what: "Latin letters", sentence: "شَرِبَ الْوَلَدُ مِنْ مَاءِ the النَّهْرِ الْبَارِدِ", problem: /Latin letters/ },
    { what: "no Arabic", sentence: "The boy drank from the cold river water", problem: /no Arabic text/ },
    { what: "no sentence", sentence: "  ", problem: /no sentence/ },
  ])("does not save a sentence with $what", async ({ sentence, problem }) => {
    answer = { ...river, sentence };
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.example).toBe("");
    expect(saved.exampleError).toMatch(problem);
  });

  it.each([
    { what: "an empty translation", translation: "", problem: /no English translation/ },
    { what: "a translation in Arabic", translation: "شَرِبَ الْوَلَدُ", problem: /translation contains Arabic/ },
  ])("does not save $what", async ({ translation, problem }) => {
    answer = { ...river, translation };
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.exampleEn).toBe("");
    expect(saved.exampleError).toMatch(problem);
  });

  it("says so when the agent only thought and gave no text", async () => {
    vi.stubGlobal("fetch", async () =>
      new Response(JSON.stringify({ outputs: [{ type: "message.output", content: [{ type: "thinking", thinking: [{ type: "text", text: "hmm" }] }] }] })));
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.exampleError).toMatch(/gave no answer/);
  });

  it("does not depend on the meanings of the answer", async () => {
    answer = { sentence: river.sentence, translation: river.translation };
    const note = context().store.createNote(cardForRank(2)!);
    const saved = (await (await askFor(note.id)).json()).note;
    expect(saved.exampleError).toBeNull();
    expect(saved.exampleEn).toBe(river.translation);
  });
});
