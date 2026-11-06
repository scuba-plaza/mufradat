import Database from "better-sqlite3";
import { randomUUID } from "node:crypto";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { audioKey, audioPath, mp3Problem } from "./audio";
import { approvalProblems, plainExample } from "./validate";
import type { AudioSource, Clip, Form, Note, NoteEdit, NewNote, POS, Slot } from "./types";

const SCHEMA = `
CREATE TABLE IF NOT EXISTS notes (
  id TEXT PRIMARY KEY,
  lemma TEXT NOT NULL,
  english TEXT NOT NULL,
  pos TEXT NOT NULL,
  gender TEXT NOT NULL DEFAULT '',
  root TEXT NOT NULL DEFAULT '',
  verb_form TEXT NOT NULL DEFAULT '',
  hint TEXT NOT NULL DEFAULT '',
  example TEXT NOT NULL DEFAULT '',
  example_en TEXT NOT NULL DEFAULT '',
  example_error TEXT,
  rank INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  lemma_source TEXT NOT NULL DEFAULT 'wiktionary',
  example_source TEXT NOT NULL DEFAULT '',
  UNIQUE (lemma, pos)
);
CREATE INDEX IF NOT EXISTS notes_rank ON notes(rank);
CREATE INDEX IF NOT EXISTS notes_status ON notes(status);
CREATE INDEX IF NOT EXISTS notes_lemma ON notes(lemma);
CREATE TABLE IF NOT EXISTS forms (
  id TEXT PRIMARY KEY,
  note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
  ord INTEGER NOT NULL,
  label TEXT NOT NULL,
  arabic TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS forms_note ON forms(note_id);
CREATE TABLE IF NOT EXISTS audio (
  note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
  slot TEXT NOT NULL,
  key TEXT,
  source TEXT,
  text TEXT NOT NULL DEFAULT '',
  error TEXT,
  PRIMARY KEY (note_id, slot)
);
`;

interface NoteRow {
  id: string; lemma: string; english: string; pos: POS; gender: string; root: string;
  verb_form: string; hint: string; example: string; example_en: string; example_error: string | null;
  rank: number; status: Note["status"]; created_at: string; updated_at: string;
  lemma_source: Note["lemmaSource"]; example_source: Note["exampleSource"];
}

interface ClipRow {
  slot: string; key: string | null; source: AudioSource | null; text: string; error: string | null;
}

export function openStore(dir: string) {
  const memory = dir === ":memory:";
  const root = memory ? fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-")) : dir;
  fs.mkdirSync(root, { recursive: true });
  const db = new Database(memory ? ":memory:" : path.join(root, "vocabulary.db"));
  db.pragma("foreign_keys = ON");
  db.exec(SCHEMA);

  const now = () => new Date().toISOString();

  function build(row: NoteRow, forms: Form[], clips: ClipRow[]): Note {
    const text: Record<Slot, string> = { word: row.lemma, example: plainExample(row.example) };
    for (const f of forms) text[f.id] = f.arabic;
    const audio: Record<Slot, Clip> = {};
    for (const c of clips) {
      if (!(c.slot in text)) continue;
      audio[c.slot] = { key: c.key, source: c.source, text: c.text, error: c.error, outdated: c.key !== null && c.text !== text[c.slot] };
    }
    return {
      id: row.id, lemma: row.lemma, english: row.english, pos: row.pos, gender: row.gender,
      root: row.root, verbForm: row.verb_form, hint: row.hint, example: row.example,
      exampleEn: row.example_en, exampleError: row.example_error, rank: row.rank,
      status: row.status, lemmaSource: row.lemma_source, exampleSource: row.example_source,
      createdAt: row.created_at, updatedAt: row.updated_at, forms, audio,
    };
  }

  function getNote(id: string): Note | null {
    const row = db.prepare("SELECT * FROM notes WHERE id = ?").get(id) as NoteRow | undefined;
    if (!row) return null;
    const forms = db.prepare("SELECT id, label, arabic FROM forms WHERE note_id = ? ORDER BY ord").all(id) as Form[];
    const clips = db.prepare("SELECT slot, key, source, text, error FROM audio WHERE note_id = ?").all(id) as ClipRow[];
    return build(row, forms, clips);
  }

  function allNotes(): Note[] {
    const group = <T extends { note_id: string }>(rows: T[]) => {
      const byNote = new Map<string, T[]>();
      for (const r of rows) {
        const list = byNote.get(r.note_id);
        if (list) list.push(r);
        else byNote.set(r.note_id, [r]);
      }
      return byNote;
    };
    const forms = group(db.prepare("SELECT note_id, id, label, arabic FROM forms ORDER BY ord").all() as (Form & { note_id: string })[]);
    const clips = group(db.prepare("SELECT note_id, slot, key, source, text, error FROM audio").all() as (ClipRow & { note_id: string })[]);
    return (db.prepare("SELECT * FROM notes").all() as NoteRow[]).map((row) =>
      build(row, (forms.get(row.id) ?? []).map(({ id, label, arabic }) => ({ id, label, arabic })), clips.get(row.id) ?? []),
    );
  }

  function createNote(input: NewNote): Note {
    const existing = db
      .prepare("SELECT 1 FROM notes WHERE lemma = ? AND pos = ?")
      .get(input.lemma, input.pos);
    if (existing) throw new Error(`There is already a card for ${input.lemma} (${input.pos}).`);
    const id = randomUUID();
    const t = now();
    db.transaction(() => {
      db.prepare(
        `INSERT INTO notes (id, lemma, english, pos, gender, root, verb_form, rank, status, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?)`,
      ).run(id, input.lemma, input.english, input.pos, input.gender ?? "", input.root ?? "",
        input.verbForm ?? "", input.rank ?? 0, t, t);
      (input.forms ?? []).forEach((f, i) =>
        db.prepare("INSERT INTO forms (id, note_id, ord, label, arabic) VALUES (?, ?, ?, ?, ?)")
          .run(randomUUID(), id, i, f.label, f.arabic),
      );
    })();
    return getNote(id)!;
  }

  /** Records a change: bumps the timestamp and takes the card back from "approved". */
  function touch(id: string): Note {
    db.prepare("UPDATE notes SET status = 'draft', updated_at = ? WHERE id = ?").run(now(), id);
    return getNote(id)!;
  }

  function need(id: string): Note {
    const n = getNote(id);
    if (!n) throw new Error("There is no such card.");
    return n;
  }

  function updateNote(id: string, edit: NoteEdit): Note {
    const before = need(id);
    db.transaction(() => {
      const set = (column: string, value: string) =>
        db.prepare(`UPDATE notes SET ${column} = ? WHERE id = ?`).run(value, id);
      if (edit.lemma !== undefined && edit.lemma !== before.lemma) {
        set("lemma", edit.lemma);
        set("lemma_source", "user");
      }
      if (edit.english !== undefined) set("english", edit.english);
      if (edit.pos !== undefined) set("pos", edit.pos);
      if (edit.gender !== undefined) set("gender", edit.gender);
      if (edit.root !== undefined) set("root", edit.root);
      if (edit.verbForm !== undefined) set("verb_form", edit.verbForm);
      if (edit.hint !== undefined) set("hint", edit.hint);
      const exampleChanged =
        (edit.example !== undefined && edit.example !== before.example) ||
        (edit.exampleEn !== undefined && edit.exampleEn !== before.exampleEn);
      if (edit.example !== undefined) set("example", edit.example);
      if (edit.example !== undefined && edit.example !== before.example) {
        db.prepare("UPDATE notes SET example_error = NULL WHERE id = ?").run(id);
      }
      if (edit.exampleEn !== undefined) set("example_en", edit.exampleEn);
      if (exampleChanged) set("example_source", "user");
      if (edit.forms) {
        const keep = new Set(edit.forms.map((f) => f.id).filter(Boolean));
        for (const f of before.forms) {
          if (keep.has(f.id)) continue;
          db.prepare("DELETE FROM forms WHERE id = ?").run(f.id);
          db.prepare("DELETE FROM audio WHERE note_id = ? AND slot = ?").run(id, f.id);
        }
        edit.forms.forEach((f, i) => {
          if (f.id && before.forms.some((b) => b.id === f.id)) {
            db.prepare("UPDATE forms SET ord = ?, label = ?, arabic = ? WHERE id = ?").run(i, f.label, f.arabic, f.id);
          } else {
            db.prepare("INSERT INTO forms (id, note_id, ord, label, arabic) VALUES (?, ?, ?, ?, ?)")
              .run(randomUUID(), id, i, f.label, f.arabic);
          }
        });
      }
    })();
    return touch(id);
  }

  function needSlot(n: Note, slot: Slot): void {
    if (!(slot === "word" || slot === "example" || n.forms.some((f) => f.id === slot))) {
      throw new Error("That audio slot does not belong to this card.");
    }
  }

  /** Stores a clip for the word, the example or one form. Failures leave the previous clip alone. */
  function setAudio(id: string, slot: Slot, bytes: Uint8Array, source: AudioSource): Note {
    const n = need(id);
    needSlot(n, slot);
    const problem = mp3Problem(bytes);
    if (problem) throw new Error(problem);
    const key = audioKey(bytes);
    const file = audioPath(root, key);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    if (!fs.existsSync(file)) fs.writeFileSync(file, bytes);
    const text = slot === "word" ? n.lemma : slot === "example" ? plainExample(n.example) : n.forms.find((f) => f.id === slot)!.arabic;
    db.prepare(
      `INSERT INTO audio (note_id, slot, key, source, text, error) VALUES (?, ?, ?, ?, ?, NULL)
       ON CONFLICT (note_id, slot) DO UPDATE SET key = excluded.key, source = excluded.source, text = excluded.text, error = NULL`,
    ).run(id, slot, key, source, text);
    return touch(id);
  }

  function removeAudio(id: string, slot: Slot): Note {
    const n = need(id);
    needSlot(n, slot);
    if (!n.audio[slot]?.key) return n;
    db.prepare("DELETE FROM audio WHERE note_id = ? AND slot = ?").run(id, slot);
    return touch(id);
  }

  /** Remembers why a generation failed, so the card still shows it later. */
  function setAudioError(id: string, slot: Slot, message: string): Note {
    need(id);
    db.prepare(
      `INSERT INTO audio (note_id, slot, error) VALUES (?, ?, ?)
       ON CONFLICT (note_id, slot) DO UPDATE SET error = excluded.error`,
    ).run(id, slot, message);
    return getNote(id)!;
  }

  function setExampleError(id: string, message: string | null): Note {
    need(id);
    db.prepare("UPDATE notes SET example_error = ? WHERE id = ?").run(message, id);
    return getNote(id)!;
  }

  /** Stores a validated example from Mistral. */
  function applyGeneratedExample(id: string, example: string, exampleEn: string): Note {
    need(id);
    db.prepare("UPDATE notes SET example = ?, example_en = ?, example_source = 'mistral', example_error = NULL WHERE id = ?")
      .run(example, exampleEn, id);
    return touch(id);
  }

  function approve(id: string): { ok: true; note: Note } | { ok: false; errors: string[] } {
    const errors = approvalProblems(need(id));
    if (errors.length > 0) return { ok: false, errors };
    db.prepare("UPDATE notes SET status = 'approved', updated_at = ? WHERE id = ?").run(now(), id);
    return { ok: true, note: getNote(id)! };
  }

  function deleteNote(id: string): void {
    db.prepare("DELETE FROM notes WHERE id = ?").run(id);
  }

  function findNote(lemma: string, pos: POS): string | null {
    const row = db.prepare("SELECT id FROM notes WHERE lemma = ? AND pos = ?").get(lemma, pos) as { id: string } | undefined;
    return row?.id ?? null;
  }

  function approvedNotes(): Note[] {
    const ids = db.prepare("SELECT id FROM notes WHERE status = 'approved'").all() as { id: string }[];
    return ids.map((r) => getNote(r.id)!);
  }

  return {
    root, findNote, approvedNotes, allNotes, close: () => db.close(), getNote, deleteNote, createNote, updateNote, setAudio, removeAudio, setAudioError, setExampleError,
    applyGeneratedExample, approve,
  };
}

export type Store = ReturnType<typeof openStore>;
