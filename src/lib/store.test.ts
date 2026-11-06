import fs from "node:fs";
import { describe, expect, it } from "vitest";
import { audioPath } from "./audio";
import { missingParts } from "./missing";
import { openStore } from "./store";

const mp3 = Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]);
const kitab = {
  lemma: "كِتَاب", english: "book", pos: "noun" as const, gender: "m", rank: 120,
  forms: [{ label: "pl.", arabic: "كُتُب" }],
};

describe("creating a bare card", () => {
  it("stores the Wiktionary data as a draft with its forms", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    expect(n.status).toBe("draft");
    expect(n.lemmaSource).toBe("wiktionary");
    expect(n.hint).toBe("");
    expect(n.forms.map((f) => [f.label, f.arabic])).toEqual([["pl.", "كُتُب"]]);
    expect(s.getNote(n.id)).toEqual(n);
  });

  it("refuses a second card for the same lemma and part of speech", () => {
    const s = openStore(":memory:");
    s.createNote(kitab);
    expect(() => s.createNote(kitab)).toThrow(/already/);
  });

  it("allows the same lemma as another part of speech", () => {
    const s = openStore(":memory:");
    s.createNote(kitab);
    expect(() => s.createNote({ ...kitab, pos: "verb" })).not.toThrow();
  });
});

describe("listing every card", () => {
  it("gives each card whole, with its forms and clips", () => {
    const s = openStore(":memory:");
    const book = s.createNote(kitab);
    const pen = s.createNote({ ...kitab, lemma: "قَلَم", english: "pen", forms: [] });
    s.setAudio(book.id, book.forms[0].id, mp3, "tts");
    s.setAudioError(pen.id, "word", "Google rejected the credentials (401).");
    const all = s.allNotes();
    expect(all.map((n) => n.lemma).sort()).toEqual(["قَلَم", "كِتَاب"].sort());
    expect(all.find((n) => n.id === book.id)!.audio[book.forms[0].id].key).not.toBeNull();
    expect(all.find((n) => n.id === pen.id)!.audio.word.error).toMatch(/401/);
    expect(all).toContainEqual(s.getNote(book.id));
    expect(all).toContainEqual(s.getNote(pen.id));
  });

  it("is empty while there is no card", () => {
    expect(openStore(":memory:").allNotes()).toEqual([]);
  });
});

describe("editing a card", () => {
  it("keeps the ids of forms that are still listed and drops the rest", () => {
    const s = openStore(":memory:");
    const n = s.createNote({ ...kitab, forms: [{ label: "pl.", arabic: "كُتُب" }, { label: "dual", arabic: "كِتَابَانِ" }] });
    const [pl, dual] = n.forms;
    const edited = s.updateNote(n.id, {
      forms: [{ id: dual.id, label: "dual", arabic: "كِتَابَيْنِ" }, { label: "x", arabic: "كُتْب" }],
    });
    expect(edited.forms.map((f) => f.arabic)).toEqual(["كِتَابَيْنِ", "كُتْب"]);
    expect(edited.forms[0].id).toBe(dual.id);
    expect(edited.forms.some((f) => f.id === pl.id)).toBe(false);
  });

  it("marks what the user changed as the user's own", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    const e = s.updateNote(n.id, { lemma: "الكِتَاب", example: "هٰذَا [كِتَابٌ]", exampleEn: "this is a book" });
    expect(e.lemmaSource).toBe("user");
    expect(e.exampleSource).toBe("user");
    expect(e.updatedAt >= n.updatedAt).toBe(true);
  });

  it("calls a card incomplete while the example or audio is missing and draft once it has all of it", () => {
    const s = openStore(":memory:");
    const n = s.createNote({ ...kitab, forms: [] });
    expect(s.updateNote(n.id, { hint: "paper" }).status).toBe("incomplete");
    s.updateNote(n.id, { example: "هٰذَا [كِتَابٌ]", exampleEn: "this is a book" });
    s.setAudio(n.id, "word", mp3, "tts");
    const done = s.setAudio(n.id, "example", mp3, "user");
    expect(done.status).toBe("draft");
  });
});

describe("approving a card", () => {
  const withExample = (s: ReturnType<typeof openStore>) => {
    const n = s.createNote(kitab);
    return s.updateNote(n.id, { example: "هٰذَا [كِتَابٌ] جَمِيلٌ", exampleEn: "this is a nice book" });
  };

  it("approves a valid card", () => {
    const s = openStore(":memory:");
    const n = withExample(s);
    const r = s.approve(n.id);
    expect(r).toMatchObject({ ok: true });
    expect(s.getNote(n.id)!.status).toBe("approved");
  });

  it("does not approve a card with a broken example and says why", () => {
    const s = openStore(":memory:");
    const n = withExample(s);
    s.updateNote(n.id, { example: "هٰذَا [كِتَابٌ] [جَمِيلٌ]" });
    const r = s.approve(n.id);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.errors.join(" ")).toMatch(/exactly one pair/);
    expect(s.getNote(n.id)!.status).not.toBe("approved");
  });

  it("does not approve a card with no meaning or a non-Arabic headword", () => {
    const s = openStore(":memory:");
    const n = s.createNote({ ...kitab, english: "", lemma: "book" });
    const r = s.approve(n.id);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.errors).toHaveLength(2);
  });

  it("needs a new approval after the card is edited", () => {
    const s = openStore(":memory:");
    const n = withExample(s);
    s.approve(n.id);
    expect(s.updateNote(n.id, { english: "a book" }).status).not.toBe("approved");
  });
});

describe("audio", () => {
  const other = Buffer.from([0xff, 0xfb, 0x90, 0x00, 9, 9, 9, 9]);

  it("keys a clip by its content and flags it outdated when the Arabic changes", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    const a = s.setAudio(n.id, "word", mp3, "tts");
    expect(a.audio.word.key).toMatch(/^[0-9a-f]{16}$/);
    expect(a.audio.word.outdated).toBe(false);
    const b = s.updateNote(n.id, { lemma: "الكِتَاب" });
    expect(b.audio.word.outdated).toBe(true);
    expect(b.audio.word.key).toBe(a.audio.word.key);
  });

  it("replaces a generated clip with an uploaded one", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    s.setAudio(n.id, "word", mp3, "tts");
    const b = s.setAudio(n.id, "word", other, "user");
    expect(b.audio.word.source).toBe("user");
  });

  it("rejects bytes that are not MP3 and keeps the clip it had", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    const a = s.setAudio(n.id, "word", mp3, "tts");
    expect(() => s.setAudio(n.id, "word", Buffer.from("hello"), "user")).toThrow(/not an MP3/);
    expect(() => s.setAudio(n.id, "word", Buffer.alloc(0), "user")).toThrow(/empty/);
    expect(s.getNote(n.id)!.audio.word.key).toBe(a.audio.word.key);
  });

  it("keeps a failure message with the card and the old clip beside it", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    const a = s.setAudio(n.id, "word", mp3, "tts");
    const failed = s.setAudioError(n.id, "word", "Google rejected the credentials (401).");
    expect(failed.audio.word.error).toMatch(/401/);
    expect(failed.audio.word.key).toBe(a.audio.word.key);
    expect(s.setAudio(n.id, "word", other, "tts").audio.word.error).toBeNull();
  });

  it("gives every form its own clip and refuses slots from other cards", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    const m = s.createNote({ ...kitab, lemma: "قَلَم" });
    const withForm = s.setAudio(n.id, n.forms[0].id, mp3, "tts");
    expect(withForm.audio[n.forms[0].id].key).not.toBeNull();
    expect(() => s.setAudio(m.id, n.forms[0].id, mp3, "tts")).toThrow(/slot/);
  });

  describe("removing a clip", () => {
    const approvedWithClips = () => {
      const s = openStore(":memory:");
      const n = s.createNote({ ...kitab, forms: [] });
      s.updateNote(n.id, { example: "هٰذَا [كِتَابٌ] جَمِيلٌ", exampleEn: "this is a nice book" });
      s.setAudio(n.id, "word", mp3, "tts");
      s.setAudio(n.id, "example", other, "tts");
      s.approve(n.id);
      return { s, id: n.id };
    };

    it("takes the card back from approved and reports the audio as missing again", () => {
      const { s, id } = approvedWithClips();
      const removed = s.removeAudio(id, "word");
      expect(removed.audio.word).toBeUndefined();
      expect(removed.audio.example.key).not.toBeNull();
      expect(removed.status).toBe("incomplete");
      expect(missingParts(removed)).toEqual(["audio for the word"]);
    });

    it("keeps the audio file, which another card may share", () => {
      const { s, id } = approvedWithClips();
      const sibling = s.createNote({ ...kitab, lemma: "قَلَم", forms: [] });
      const key = s.setAudio(sibling.id, "word", mp3, "tts").audio.word.key!;
      s.removeAudio(id, "word");
      expect(fs.existsSync(audioPath(s.root, key))).toBe(true);
      expect(s.getNote(sibling.id)!.audio.word.key).toBe(key);
    });

    it("forgets a failure message together with the clip", () => {
      const { s, id } = approvedWithClips();
      s.setAudioError(id, "word", "Google rejected the credentials (401).");
      expect(s.removeAudio(id, "word").audio.word).toBeUndefined();
    });

    it("changes nothing when the slot has no clip", () => {
      const { s, id } = approvedWithClips();
      const empty = s.createNote({ ...kitab, lemma: "قَلَم", forms: [] });
      s.updateNote(empty.id, { example: "هٰذَا [قَلَمٌ] جَمِيلٌ", exampleEn: "this is a nice pen" });
      s.approve(empty.id);
      const before = s.getNote(empty.id)!;
      expect(s.removeAudio(empty.id, "word")).toEqual(before);
      expect(s.getNote(id)!.status).toBe("approved");
    });

    it("refuses a slot that belongs to another card", () => {
      const s = openStore(":memory:");
      const n = s.createNote(kitab);
      const m = s.createNote({ ...kitab, lemma: "قَلَم" });
      s.setAudio(n.id, n.forms[0].id, mp3, "tts");
      expect(() => s.removeAudio(m.id, n.forms[0].id)).toThrow("That audio slot does not belong to this card.");
      expect(s.getNote(n.id)!.audio[n.forms[0].id].key).not.toBeNull();
    });
  });

  it("keeps a failed example generation visible and clears it on success", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    expect(s.setExampleError(n.id, "Mistral is busy (429).").exampleError).toMatch(/429/);
    const ok = s.applyGeneratedExample(n.id, "هٰذَا [كِتَابٌ]", "this is a book");
    expect(ok.exampleError).toBeNull();
    expect(ok.exampleSource).toBe("mistral");
  });

  it("drops a failed example generation once you write an example yourself", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    s.setExampleError(n.id, "Mistral's example was not saved. The example has 2 words; it must have 3 to 9.");
    const written = s.updateNote(n.id, { example: "هٰذَا [كِتَابٌ] جَدِيدٌ" });
    expect(written.exampleError).toBeNull();
    expect(written.exampleSource).toBe("user");
  });
});
