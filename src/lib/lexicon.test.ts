import { describe, expect, it } from "vitest";
import { cardForRank, lookup, rankingSize } from "./lexicon";

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
});
