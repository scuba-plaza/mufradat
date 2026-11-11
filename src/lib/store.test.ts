import Database from "better-sqlite3";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { audioKey, audioPath } from "./audio";
import { missingParts } from "./missing";
import { openStore, type Store } from "./store";

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
    expect(n.hintAr).toBe("");
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

  it("keeps a card that is not approved a draft, whatever it still lacks", () => {
    const s = openStore(":memory:");
    const n = s.createNote({ ...kitab, forms: [] });
    expect(s.updateNote(n.id, { hint: "paper", hintAr: "ليس كَتَب" }).status).toBe("draft");
    expect(s.getNote(n.id)!.hintAr).toBe("ليس كَتَب");
    s.updateNote(n.id, { example: "هٰذَا [كِتَابٌ]", exampleEn: "this is a book" });
    s.setAudio(n.id, "word", mp3, "tts");
    expect(s.setAudio(n.id, "example", mp3, "user").status).toBe("draft");
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

describe("tagging a card", () => {
  it("keeps a card's tags, ascending, with the card", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    expect(n.tags).toEqual([]);
    expect(s.setTags(n.id, ["b", "a"]).tags).toEqual(["a", "b"]);
    expect(s.getNote(n.id)!.tags).toEqual(["a", "b"]);
    expect(s.allNotes()[0].tags).toEqual(["a", "b"]);
  });

  describe("an approved card", () => {
    afterEach(() => vi.useRealTimers());

    it("stays approved and keeps its edited time", () => {
      vi.useFakeTimers({ toFake: ["Date"] });
      vi.setSystemTime(new Date("2026-11-01T10:00:00.000Z"));
      const s = openStore(":memory:");
      const n = s.createNote(kitab);
      s.updateNote(n.id, { example: "هٰذَا [كِتَابٌ] جَمِيلٌ", exampleEn: "this is a nice book" });
      s.approve(n.id);
      vi.setSystemTime(new Date("2026-11-02T10:00:00.000Z"));
      const tagged = s.setTags(n.id, ["lesson_1"]);
      expect(tagged.status).toBe("approved");
      expect(tagged.updatedAt).toBe("2026-11-01T10:00:00.000Z");
    });
  });

  it("refuses an unknown card", () => {
    expect(() => openStore(":memory:").setTags("no-such-card", ["a"])).toThrow("There is no such card.");
  });

  it("resets a tagged card", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    s.setTags(n.id, ["a"]);
    s.deleteNote(n.id);
    expect(s.getNote(n.id)).toBeNull();
    const file = path.join(fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-tags-")), "copy.db");
    s.exportDatabase(file);
    const raw = new Database(file, { readonly: true });
    expect(raw.prepare("SELECT count(*) AS n FROM note_tags").get()).toEqual({ n: 0 });
    raw.close();
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
      expect(removed.status).toBe("draft");
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
    s.setExampleError(n.id, "Mistral's example was not saved. The sentence has words without a vowel mark: كِتَاب.");
    const written = s.updateNote(n.id, { example: "هٰذَا [كِتَابٌ] جَدِيدٌ" });
    expect(written.exampleError).toBeNull();
    expect(written.exampleSource).toBe("user");
  });
});

describe("deleting a card", () => {
  it("removes the card and leaves the other cards as they were", () => {
    const s = openStore(":memory:");
    const gone = s.createNote(kitab);
    const kept = s.createNote({ ...kitab, lemma: "قَلَم", english: "pen", rank: 121 });
    s.setAudio(gone.id, "word", mp3, "tts");
    s.setAudio(kept.id, "word", mp3, "tts");
    s.deleteNote(gone.id);
    expect(s.getNote(gone.id)).toBeNull();
    expect(s.findNote(kitab.lemma, kitab.pos)).toBeNull();
    expect(s.allNotes().map((n) => n.id)).toEqual([kept.id]);
    expect(s.getNote(kept.id)!.audio.word.key).not.toBeNull();
  });

  it("leaves no form or clip of the card behind in the workspace", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-delete-"));
    try {
      const s = openStore(dir);
      const n = s.createNote(kitab);
      s.setAudio(n.id, "word", mp3, "tts");
      s.setAudioError(n.id, n.forms[0].id, "Google rejected the credentials (401).");
      s.deleteNote(n.id);
      s.close();
      const db = new Database(path.join(dir, "vocabulary.db"), { readonly: true });
      const count = (table: string) => (db.prepare(`SELECT COUNT(*) AS n FROM ${table}`).get() as { n: number }).n;
      expect([count("notes"), count("forms"), count("audio")]).toEqual([0, 0, 0]);
      db.close();
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  it("lets the same word be started again from nothing", () => {
    const s = openStore(":memory:");
    const first = s.createNote(kitab);
    s.updateNote(first.id, { hint: "paper", hintAr: "ليس كَتَب", example: "هٰذَا [كِتَابٌ]", exampleEn: "this is a book" });
    s.setAudio(first.id, "word", mp3, "tts");
    s.deleteNote(first.id);
    const again = s.createNote(kitab);
    expect(again.id).not.toBe(first.id);
    expect(again).toMatchObject({ hint: "", hintAr: "", example: "", exampleEn: "", status: "draft", audio: {} });
  });

  it("does nothing for a card that is already gone", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    s.deleteNote(n.id);
    expect(() => s.deleteNote(n.id)).not.toThrow();
    expect(() => s.deleteNote("no-such-card")).not.toThrow();
  });
});

describe("audio files that no clip uses", () => {
  const bytes = (n: number) => Buffer.from([0xff, 0xfb, 0x90, 0x00, n, n, n, n]);
  const fileOf = (dir: string, content: Buffer) => audioPath(dir, audioKey(content));

  function inWorkspace(run: (s: Store, dir: string) => void) {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-unused-"));
    try {
      const s = openStore(dir);
      run(s, dir);
      s.close();
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  }

  it("reports the file a removed clip left behind, deletes it, and then finds nothing", () => {
    inWorkspace((s, dir) => {
      const n = s.createNote(kitab);
      s.setAudio(n.id, "word", bytes(1), "tts");
      expect(s.unusedAudio()).toEqual({ files: 0, bytes: 0 });
      s.removeAudio(n.id, "word");
      expect(s.unusedAudio()).toEqual({ files: 1, bytes: 8 });
      expect(fs.existsSync(fileOf(dir, bytes(1)))).toBe(true);
      expect(s.deleteUnusedAudio()).toEqual({ files: 1, bytes: 8 });
      expect(fs.existsSync(fileOf(dir, bytes(1)))).toBe(false);
      expect(s.unusedAudio()).toEqual({ files: 0, bytes: 0 });
      expect(s.deleteUnusedAudio()).toEqual({ files: 0, bytes: 0 });
    });
  });

  it("keeps the file of a clip whose text has changed since it was made", () => {
    inWorkspace((s, dir) => {
      const n = s.createNote(kitab);
      s.setAudio(n.id, "word", bytes(1), "tts");
      s.updateNote(n.id, { lemma: "كُتُب" });
      expect(s.getNote(n.id)!.audio.word.outdated).toBe(true);
      expect(s.deleteUnusedAudio()).toEqual({ files: 0, bytes: 0 });
      expect(fs.existsSync(fileOf(dir, bytes(1)))).toBe(true);
    });
  });

  it("finds the file that a replaced clip left behind and keeps the new one", () => {
    inWorkspace((s, dir) => {
      const n = s.createNote(kitab);
      s.setAudio(n.id, "word", bytes(1), "tts");
      s.setAudio(n.id, "word", bytes(2), "user");
      expect(s.deleteUnusedAudio()).toEqual({ files: 1, bytes: 8 });
      expect(fs.existsSync(fileOf(dir, bytes(1)))).toBe(false);
      expect(fs.existsSync(fileOf(dir, bytes(2)))).toBe(true);
      expect(s.getNote(n.id)!.audio.word.key).toBe(audioKey(bytes(2)));
    });
  });

  it("keeps a file that two clips share until neither has it", () => {
    inWorkspace((s, dir) => {
      const a = s.createNote(kitab);
      const b = s.createNote({ ...kitab, lemma: "قَلَم", english: "pen", rank: 121 });
      s.setAudio(a.id, "word", bytes(1), "tts");
      s.setAudio(b.id, "word", bytes(1), "tts");
      s.removeAudio(a.id, "word");
      expect(s.deleteUnusedAudio()).toEqual({ files: 0, bytes: 0 });
      expect(fs.existsSync(fileOf(dir, bytes(1)))).toBe(true);
      s.removeAudio(b.id, "word");
      expect(s.deleteUnusedAudio()).toEqual({ files: 1, bytes: 8 });
    });
  });

  it("finds the files that a removed form and a deleted card left behind", () => {
    inWorkspace((s, dir) => {
      const n = s.createNote(kitab);
      s.setAudio(n.id, n.forms[0].id, bytes(3), "tts");
      s.setAudio(n.id, "word", bytes(4), "tts");
      s.updateNote(n.id, { forms: [] });
      expect(s.unusedAudio()).toEqual({ files: 1, bytes: 8 });
      s.deleteNote(n.id);
      expect(s.unusedAudio()).toEqual({ files: 2, bytes: 16 });
      expect(s.deleteUnusedAudio()).toEqual({ files: 2, bytes: 16 });
      expect(fs.existsSync(fileOf(dir, bytes(3)))).toBe(false);
      expect(fs.existsSync(fileOf(dir, bytes(4)))).toBe(false);
    });
  });

  it("leaves alone whatever is not an audio file of its own", () => {
    inWorkspace((s, dir) => {
      const stray = ["notes.txt", "ab/notes.txt", "ab/abcdef0123456789.wav", "cd/0123456789abcdef.mp3", "ab/ABCDEF0123456789.mp3", "ab/abcdef01234567.mp3"].map((name) =>
        path.join(dir, "audio", name),
      );
      for (const file of stray) {
        fs.mkdirSync(path.dirname(file), { recursive: true });
        fs.writeFileSync(file, "x");
      }
      expect(s.unusedAudio()).toEqual({ files: 0, bytes: 0 });
      expect(s.deleteUnusedAudio()).toEqual({ files: 0, bytes: 0 });
      expect(stray.every((file) => fs.existsSync(file))).toBe(true);
    });
  });

  it("finds nothing, without failing, in a workspace that has no audio folder yet", () => {
    inWorkspace((s, dir) => {
      expect(fs.existsSync(path.join(dir, "audio"))).toBe(false);
      expect(s.unusedAudio()).toEqual({ files: 0, bytes: 0 });
      expect(s.deleteUnusedAudio()).toEqual({ files: 0, bytes: 0 });
    });
  });
});

describe("suspending a card", () => {
  const withExample = (s: ReturnType<typeof openStore>) => {
    const n = s.createNote(kitab);
    return s.updateNote(n.id, { example: "\u0647\u0670\u0630\u064e\u0627 [\u0643\u0650\u062a\u064e\u0627\u0628\u064c] \u062c\u064e\u0645\u0650\u064a\u0644\u064c", exampleEn: "this is a nice book" });
  };

  it("marks the card suspended and takes its approval back", () => {
    const s = openStore(":memory:");
    const n = withExample(s);
    s.approve(n.id);
    const suspended = s.suspend(n.id);
    expect(suspended.suspended).toBe(true);
    expect(suspended.status).toBe("draft");
    expect(s.getNote(n.id)).toMatchObject({ suspended: true, status: "draft" });
  });

  it("unsuspends the card and leaves it a draft that still lacks nothing", () => {
    const s = openStore(":memory:");
    const n = withExample(s);
    s.suspend(n.id);
    const back = s.unsuspend(n.id);
    expect(back.suspended).toBe(false);
    expect(back.status).toBe("draft");
    expect(s.getNote(n.id)!.suspended).toBe(false);
  });

  it("says there is no such card", () => {
    const s = openStore(":memory:");
    expect(() => s.suspend("no-such-card")).toThrow(/no such card/i);
    expect(() => s.unsuspend("no-such-card")).toThrow(/no such card/i);
  });

  it("refuses to approve a suspended card", () => {
    const s = openStore(":memory:");
    const n = withExample(s);
    s.suspend(n.id);
    const r = s.approve(n.id);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.errors.join(" ")).toMatch(/suspend/i);
    expect(s.getNote(n.id)!.status).toBe("draft");
  });

  it("keeps a card suspended while it is edited", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    s.suspend(n.id);
    expect(s.updateNote(n.id, { english: "a book" }).suspended).toBe(true);
    expect(s.setTags(n.id, ["a"]).suspended).toBe(true);
  });

  it("gives a database made before suspension cards that are not suspended", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-suspend-"));
    try {
      const first = openStore(dir);
      const card = first.createNote(kitab);
      first.close();
      const raw = new Database(path.join(dir, "vocabulary.db"));
      raw.exec("ALTER TABLE notes DROP COLUMN suspended");
      raw.pragma("user_version = 2");
      raw.close();
      const again = openStore(dir);
      expect(again.getNote(card.id)!.suspended).toBe(false);
      expect(again.suspend(card.id).suspended).toBe(true);
      again.close();
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});

describe("the workspace version", () => {
  function inDir(run: (dir: string) => void) {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-version-"));
    try {
      run(dir);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  }

  const setVersion = (dir: string, version: number) => {
    const raw = new Database(path.join(dir, "vocabulary.db"));
    raw.pragma(`user_version = ${version}`);
    raw.close();
  };

  it("refuses a workspace written by a newer version", () => {
    inDir((dir) => {
      setVersion(dir, 99);
      expect(() => openStore(dir)).toThrow("This workspace was written by a newer Mufradat.");
    });
  });

  it("opens a workspace from before the version existed and keeps its cards", () => {
    inDir((dir) => {
      const first = openStore(dir);
      const card = first.createNote(kitab);
      first.close();
      setVersion(dir, 0);
      const again = openStore(dir);
      expect(again.getNote(card.id)?.lemma).toBe("كِتَاب");
      again.close();
    });
  });

  it("opens a database made before the Arabic hint and gives its cards an empty one", () => {
    inDir((dir) => {
      const first = openStore(dir);
      const card = first.createNote(kitab);
      first.close();
      const raw = new Database(path.join(dir, "vocabulary.db"));
      raw.exec("ALTER TABLE notes DROP COLUMN hint_ar");
      raw.pragma("user_version = 2");
      raw.close();
      const again = openStore(dir);
      expect(again.getNote(card.id)!.hintAr).toBe("");
      expect(again.updateNote(card.id, { hintAr: "ليس كَتَب" }).hintAr).toBe("ليس كَتَب");
      again.close();
    });
  });

  it("opens a database made before tags and gives its cards no tags", () => {
    inDir((dir) => {
      const first = openStore(dir);
      const card = first.createNote(kitab);
      first.close();
      const raw = new Database(path.join(dir, "vocabulary.db"));
      raw.exec("DROP TABLE note_tags");
      raw.pragma("user_version = 1");
      raw.close();
      const again = openStore(dir);
      expect(again.getNote(card.id)!.tags).toEqual([]);
      expect(again.setTags(card.id, ["a"]).tags).toEqual(["a"]);
      again.close();
    });
  });
});

describe("checking the workspace", () => {
  it("reports a healthy workspace as healthy", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    s.setAudio(n.id, "word", mp3, "tts");
    expect(s.check()).toEqual({ cards: 1, approved: 0, integrity: "ok", missingAudio: [] });
  });

  it("reports a clip whose file is gone with its card and slot", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    s.setAudio(n.id, "word", mp3, "tts");
    fs.rmSync(audioPath(s.root, audioKey(mp3)));
    expect(s.check().missingAudio).toEqual([
      { noteId: n.id, lemma: "كِتَاب", slot: "word", slotName: "word", key: audioKey(mp3) },
    ]);
  });

  it("names a form's clip by the form's label", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    const form = n.forms[0];
    s.setAudio(n.id, form.id, mp3, "tts");
    fs.rmSync(audioPath(s.root, audioKey(mp3)));
    expect(s.check().missingAudio.map((m) => [m.slot, m.slotName])).toEqual([[form.id, 'form "pl."']]);
  });

  it("counts the cards and the approved ones", () => {
    const s = openStore(":memory:");
    const n = s.createNote(kitab);
    s.createNote({ ...kitab, lemma: "قَلَم", english: "pen", forms: [] });
    s.approve(n.id);
    expect(s.counts()).toEqual({ cards: 2, approved: 1 });
  });
});
