import Database from "better-sqlite3";
import { randomUUID } from "node:crypto";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { audioFiles, audioKey, audioPath, mp3Problem, slotLabel, type AudioFile } from "./audio";
import { approvalProblems, plainExample } from "./validate";
import type { AudioSource, Clip, Form, Note, NoteEdit, NewNote, POS, Slot, WorkspaceCheck } from "./types";

export const SCHEMA_VERSION = 3;

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
  suspended INTEGER NOT NULL DEFAULT 0,
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
CREATE TABLE IF NOT EXISTS note_tags (
  note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
  tag TEXT NOT NULL,
  PRIMARY KEY (note_id, tag)
);
`;

interface NoteRow {
  id: string; lemma: string; english: string; pos: POS; gender: string; root: string;
  verb_form: string; hint: string; example: string; example_en: string; example_error: string | null;
  rank: number; status: Note["status"]; suspended: number; created_at: string; updated_at: string;
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
  const found = db.pragma("user_version", { simple: true }) as number;
  if (found > SCHEMA_VERSION) {
    db.close();
    throw new Error("This workspace was written by a newer Mufradat.");
  }
  db.pragma("foreign_keys = ON");
  db.exec(SCHEMA);
  if (found < SCHEMA_VERSION) {
    const columns = db.prepare("PRAGMA table_info(notes)").all() as { name: string }[];
    if (!columns.some((c) => c.name === "suspended")) db.exec("ALTER TABLE notes ADD COLUMN suspended INTEGER NOT NULL DEFAULT 0");
    db.pragma(`user_version = ${SCHEMA_VERSION}`);
  }

  const now = () => new Date().toISOString();

  function build(row: NoteRow, forms: Form[], clips: ClipRow[], tags: string[]): Note {
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
      status: row.status, suspended: row.suspended === 1, lemmaSource: row.lemma_source, exampleSource: row.example_source,
      createdAt: row.created_at, updatedAt: row.updated_at, forms, audio, tags,
    };
  }

  function getNote(id: string): Note | null {
    const row = db.prepare("SELECT * FROM notes WHERE id = ?").get(id) as NoteRow | undefined;
    if (!row) return null;
    const forms = db.prepare("SELECT id, label, arabic FROM forms WHERE note_id = ? ORDER BY ord").all(id) as Form[];
    const clips = db.prepare("SELECT slot, key, source, text, error FROM audio WHERE note_id = ?").all(id) as ClipRow[];
    const tags = (db.prepare("SELECT tag FROM note_tags WHERE note_id = ? ORDER BY tag").all(id) as { tag: string }[]).map((r) => r.tag);
    return build(row, forms, clips, tags);
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
    const tags = group(db.prepare("SELECT note_id, tag FROM note_tags ORDER BY tag").all() as { note_id: string; tag: string }[]);
    return (db.prepare("SELECT * FROM notes").all() as NoteRow[]).map((row) =>
      build(
        row,
        (forms.get(row.id) ?? []).map(({ id, label, arabic }) => ({ id, label, arabic })),
        clips.get(row.id) ?? [],
        (tags.get(row.id) ?? []).map((t) => t.tag),
      ),
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

  function setTags(id: string, tags: readonly string[]): Note {
    need(id);
    const insert = db.prepare("INSERT INTO note_tags (note_id, tag) VALUES (?, ?)");
    db.transaction(() => {
      db.prepare("DELETE FROM note_tags WHERE note_id = ?").run(id);
      for (const tag of tags) insert.run(id, tag);
    })();
    return getNote(id)!;
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
    const before = need(id);
    if (before.suspended) return { ok: false, errors: ["Unsuspend the card before approving it."] };
    const errors = approvalProblems(before);
    if (errors.length > 0) return { ok: false, errors };
    db.prepare("UPDATE notes SET status = 'approved', updated_at = ? WHERE id = ?").run(now(), id);
    return { ok: true, note: getNote(id)! };
  }

  /** Parks a card: it takes the approval back and is found only by status:suspended. */
  function suspend(id: string): Note {
    need(id);
    db.prepare("UPDATE notes SET suspended = 1, status = 'draft', updated_at = ? WHERE id = ?").run(now(), id);
    return getNote(id)!;
  }

  function unsuspend(id: string): Note {
    need(id);
    db.prepare("UPDATE notes SET suspended = 0, updated_at = ? WHERE id = ?").run(now(), id);
    return getNote(id)!;
  }

  function deleteNote(id: string): void {
    db.prepare("DELETE FROM notes WHERE id = ?").run(id);
  }

  function findUnusedAudio(): AudioFile[] {
    const rows = db.prepare("SELECT DISTINCT key FROM audio WHERE key IS NOT NULL").all() as { key: string }[];
    const used = new Set(rows.map((r) => r.key));
    return audioFiles(root).filter((file) => !used.has(file.key));
  }

  const totals = (files: AudioFile[]) => ({ files: files.length, bytes: files.reduce((sum, file) => sum + file.bytes, 0) });

  function unusedAudio(): { files: number; bytes: number } {
    return totals(findUnusedAudio());
  }

  function deleteUnusedAudio(): { files: number; bytes: number } {
    const unused = findUnusedAudio();
    for (const file of unused) fs.rmSync(file.path, { force: true });
    return totals(unused);
  }

  function findNote(lemma: string, pos: POS): string | null {
    const row = db.prepare("SELECT id FROM notes WHERE lemma = ? AND pos = ?").get(lemma, pos) as { id: string } | undefined;
    return row?.id ?? null;
  }

  function audioKeys(): string[] {
    return (db.prepare("SELECT DISTINCT key FROM audio WHERE key IS NOT NULL").all() as { key: string }[]).map((r) => r.key);
  }

  function exportDatabase(dest: string): void {
    db.prepare("VACUUM INTO ?").run(dest);
  }

  function counts(): { cards: number; approved: number } {
    return db
      .prepare("SELECT COUNT(*) AS cards, COALESCE(SUM(status = 'approved'), 0) AS approved FROM notes")
      .get() as { cards: number; approved: number };
  }

  function check(): WorkspaceCheck {
    const rows = db
      .prepare(
        `SELECT a.note_id AS noteId, n.lemma AS lemma, a.slot AS slot, a.key AS key
         FROM audio a JOIN notes n ON n.id = a.note_id WHERE a.key IS NOT NULL`,
      )
      .all() as { noteId: string; lemma: string; slot: Slot; key: string }[];
    const missingAudio = rows
      .filter((r) => !fs.existsSync(audioPath(root, r.key)))
      .map((r) => ({ ...r, slotName: slotLabel(getNote(r.noteId)!.forms, r.slot) }));
    return { ...counts(), integrity: db.pragma("integrity_check", { simple: true }) as string, missingAudio };
  }

  return {
    root, findNote, allNotes, counts, check, exportDatabase, audioKeys, isOpen: () => db.open, close: () => db.close(), getNote, deleteNote, unusedAudio, deleteUnusedAudio, createNote, updateNote, setTags, setAudio, removeAudio, setAudioError, setExampleError,
    applyGeneratedExample, approve, suspend, unsuspend,
  };
}

export type Store = ReturnType<typeof openStore>;
