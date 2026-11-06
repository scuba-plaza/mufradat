import { describe, expect, it } from "vitest";
import { cardForRank, lookup, rankedWords, rankingSize, wiktionaryWords } from "./lexicon";

describe("the embedded Wiktionary data", () => {
  it("has the 5000-word list and a bare card for each of its first words", () => {
    expect(rankingSize()).toBe(5000);
    expect(cardForRank(1)).toMatchObject({ lemma: "فِي", pos: "prep", english: "in, within", rank: 1 });
    expect(cardForRank(5001)).toBeNull();
    for (let r = 1; r <= 5000; r++) expect(cardForRank(r), `rank ${r}`).not.toBeNull();
  });

  it("finds a word typed without vowels, with its other parts of speech unranked", () => {
    const found = lookup("على");
    expect(found.find((c) => c.pos === "prep")).toMatchObject({ rank: 3 });
    const verb = found.find((c) => c.pos === "verb")!;
    expect(verb).toMatchObject({ rank: 0, verbForm: "I" });
    expect(verb.forms).toContainEqual({ label: "pres.", arabic: "يَعْلِي" });
  });

  it("tells the part of speech of the card, gender, root and verb form of each frequency word", () => {
    const words = rankedWords();
    expect(words[0]).toEqual({ rank: 1, lemma: "فِي", english: "in, within", pos: "prep", gender: "", root: "", verbForm: "" });
    expect(words[5]).toMatchObject({ lemma: "كَانَ", pos: "verb", verbForm: "I" });
    expect(words[67]).toMatchObject({ lemma: "أَحَد", pos: "noun", gender: "m", root: "ء ح د" });
    expect(words[211]).toMatchObject({ rank: 212, pos: "det" });
  });

  it("lists every Wiktionary entry once for its lemma and part of speech, ranked only where the ranking lists it", () => {
    const entries = wiktionaryWords();
    const on = entries.filter((e) => e.lemma === "عَلَى");
    expect(on).toEqual([
      { lemma: "عَلَى", english: "on, over", pos: "prep", gender: "", root: "", verbForm: "", rank: 3 },
      { lemma: "عَلَى", english: "to climb", pos: "verb", gender: "", root: "", verbForm: "I", rank: 0 },
    ]);
    expect(entries.filter((e) => e.lemma === "آنَ")).toHaveLength(1);
    expect(entries.length).toBeGreaterThan(20000);
  });
});
