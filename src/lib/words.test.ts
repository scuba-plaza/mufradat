import { describe, expect, it } from "vitest";
import { openCard, frequencyList, nextWord } from "./words";
import { openStore } from "./store";

describe("the frequency list", () => {
  it("lists all 5000 words in rank order with lemma and English", () => {
    const list = frequencyList(openStore(":memory:"));
    expect(list).toHaveLength(5000);
    expect(list[0]).toEqual({ rank: 1, lemma: "فِي", english: "in, within" });
    expect(list.map((w) => w.rank)).toEqual(list.map((_, i) => i + 1));
  });

  it("shows what you wrote on a card instead of the Wiktionary text", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { rank: 1 })!;
    s.updateNote(id, { english: "in (a place)" });
    expect(frequencyList(s)[0].english).toBe("in (a place)");
    expect(frequencyList(s)[1].english).toBe("having partitive effect: of, some of, parts of, one of");
  });

  it("gives a word the status of its card, and a word without a card none", () => {
    const s = openStore(":memory:");
    expect(frequencyList(s)[0]).not.toHaveProperty("status");
    const bare = openCard(s, { rank: 1 })!;
    const edited = openCard(s, { rank: 2 })!;
    const approved = openCard(s, { rank: 3 })!;
    s.updateNote(edited, { hint: "a hint" });
    expect(s.approve(approved).ok).toBe(true);
    expect(s.getNote(bare)!.status).toBe("draft");
    expect(frequencyList(s).slice(0, 4).map((w) => w.status)).toEqual(["draft", "incomplete", "approved", undefined]);
  });
});

describe("opening a word", () => {
  it("creates the card the first time and finds it afterwards", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { rank: 3 })!;
    expect(s.getNote(id)).toMatchObject({ lemma: "عَلَى", pos: "prep", rank: 3 });
    expect(openCard(s, { rank: 3 })).toBe(id);
    expect(s.rankedCards().size).toBe(1);
  });

  it("opens a Wiktionary entry that is not in the list as an unranked card", () => {
    const s = openStore(":memory:");
    const id = openCard(s, { lemma: "عَلَى", pos: "verb" })!;
    expect(s.getNote(id)).toMatchObject({ pos: "verb", rank: 0 });
  });

  it("opens the entry that was chosen when other entries share its letters and part of speech", () => {
    const s = openStore(":memory:");
    const chosen = [
      { lemma: "إِن", pos: "conj" },
      { lemma: "أَنْ", pos: "conj" },
      { lemma: "آنَ", pos: "verb" },
    ];
    const ids = chosen.map((target) => openCard(s, target)!);
    expect(ids.map((id) => s.getNote(id)!.lemma)).toEqual(chosen.map((target) => target.lemma));
    expect(new Set(ids).size).toBe(chosen.length);
  });

  it("says so when there is no such word", () => {
    const s = openStore(":memory:");
    expect(openCard(s, { rank: 5001 })).toBeNull();
    expect(openCard(s, { lemma: "zzz", pos: "noun" })).toBeNull();
    expect(openCard(s, { lemma: "على", pos: "nonsense" })).toBeNull();
    expect(s.rankedCards().size).toBe(0);
  });
});

describe("the next word", () => {
  it("is the first word whose card is not approved, and moves on when one is skipped", () => {
    const s = openStore(":memory:");
    expect(nextWord(s)).toEqual({ rank: 1, lemma: "فِي", english: "in, within" });
    const first = openCard(s, { rank: 1 })!;
    expect(nextWord(s)).toMatchObject({ rank: 1, status: "draft" });
    s.approve(first);
    expect(nextWord(s)!.rank).toBe(2);
    s.skipRank(2);
    expect(nextWord(s)!.rank).toBe(3);
  });
});
