import Database from "better-sqlite3";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { audioKey, audioPath } from "./audio";
import { backupZipStream } from "./backup-zip";
import { backupEntries, createSnapshot, dailyDue, deleteSnapshot, importBackup, listSnapshots, restoreSnapshot, snapshotAudioPath, snapshotUsage } from "./snapshots";
import type { SnapshotReason } from "./types";
import { openStore } from "./store";

const mp3 = Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]);
const kitab = {
  lemma: "كِتَاب", english: "book", pos: "noun" as const, gender: "m", rank: 120,
  forms: [{ label: "pl.", arabic: "كُتُب" }],
};

const roots: string[] = [];

afterEach(() => {
  for (const root of roots.splice(0)) fs.rmSync(root, { recursive: true, force: true });
});

function workspace() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-snapshots-"));
  roots.push(root);
  return { root, store: openStore(root) };
}

const at = (iso: string) => ({ now: new Date(iso) });

describe("restoring a snapshot", () => {
  it("brings a reset card back with its forms", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    store.deleteNote(card.id);
    restoreSnapshot(store, snapshot.id);
    const back = openStore(root);
    expect(back.getNote(card.id)?.forms.map((f) => [f.label, f.arabic])).toEqual([["pl.", "كُتُب"]]);
  });

  it("plays the card's clip after a restore, also after Prune audio", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    store.setAudio(card.id, "word", mp3, "tts");
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    store.deleteNote(card.id);
    store.deleteUnusedAudio();
    restoreSnapshot(store, snapshot.id);
    const back = openStore(root);
    const key = back.getNote(card.id)!.audio.word.key!;
    expect(fs.readFileSync(audioPath(root, key))).toEqual(mp3);
  });

  it("brings every clip back when the whole audio folder is gone", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    store.setAudio(card.id, "word", mp3, "tts");
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    fs.rmSync(path.join(root, "audio"), { recursive: true });
    restoreSnapshot(store, snapshot.id);
    const back = openStore(root);
    const key = back.getNote(card.id)!.audio.word.key!;
    expect(fs.readFileSync(audioPath(root, key))).toEqual(mp3);
  });

  it("snapshots and restores a workspace that has cards and no audio folder at all", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    expect(snapshot.clips).toBe(0);
    store.deleteNote(card.id);
    restoreSnapshot(store, snapshot.id);
    expect(openStore(root).getNote(card.id)?.lemma).toBe("كِتَاب");
  });
});

describe("a restore", () => {
  it("brings the workspace back and takes no snapshot of the state it replaces", () => {
    const { root, store } = workspace();
    store.createNote(kitab);
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    const second = store.createNote({ ...kitab, lemma: "قَلَم", english: "pen", forms: [] });
    restoreSnapshot(store, snapshot.id, at("2026-11-06T11:00:00Z"));
    const afterRestore = openStore(root);
    expect(afterRestore.getNote(second.id)).toBeNull();
    expect(listSnapshots(root).map((s) => s.reason)).toEqual(["manual"]);
  });
});

describe("refusing a restore", () => {
  const clipsGone = "The backup is missing or has damaged audio clips.";
  const tamperings: [string, (root: string, id: string, key: string) => void, string][] = [
    [
      "a snapshot from a newer version",
      (root, id) => {
        const file = path.join(root, "snapshots", id, "manifest.json");
        fs.writeFileSync(file, JSON.stringify({ ...JSON.parse(fs.readFileSync(file, "utf8")), schema: 99 }));
      },
      "This backup was written by a newer Mufradat.",
    ],
    [
      "a damaged database",
      (root, id) => fs.writeFileSync(path.join(root, "snapshots", id, "vocabulary.db"), "not a database"),
      "The backup's database is damaged.",
    ],
    ["a damaged clip", (root, _id, key) => fs.writeFileSync(snapshotAudioPath(root, key), Buffer.from("x")), clipsGone],
    ["a missing clip", (root, _id, key) => fs.rmSync(snapshotAudioPath(root, key)), clipsGone],
  ];

  it.each(tamperings)("refuses %s and changes nothing", (_name, tamper, message) => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    store.setAudio(card.id, "word", mp3, "tts");
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    const later = store.createNote({ ...kitab, lemma: "قَلَم", english: "pen", forms: [] });
    tamper(root, snapshot.id, store.getNote(card.id)!.audio.word.key!);
    expect(() => restoreSnapshot(store, snapshot.id)).toThrow(message);
    expect(store.getNote(later.id)?.lemma).toBe("قَلَم");
    expect(listSnapshots(root).map((s) => s.id)).toEqual([snapshot.id]);
  });

  it.each(["../x", "20260101T000000Z-manual"])("refuses %s as a snapshot that does not exist", (id) => {
    const { store } = workspace();
    store.createNote(kitab);
    createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    expect(() => restoreSnapshot(store, id)).toThrow("There is no such backup.");
  });

  it("refuses an id that walks out of the snapshot folder into a real one", () => {
    const { store } = workspace();
    store.createNote(kitab);
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    expect(() => restoreSnapshot(store, `../snapshots/${snapshot.id}`)).toThrow("There is no such backup.");
  });
});

describe("taking a snapshot of an imperfect workspace", () => {
  it("still takes the snapshot when clips are missing or damaged, and counts them", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    const other = Buffer.from([0xff, 0xfb, 0x90, 0x00, 9, 9, 9, 9]);
    store.setAudio(card.id, "word", mp3, "tts");
    store.setAudio(card.id, card.forms[0].id, other, "tts");
    fs.rmSync(audioPath(root, audioKey(mp3)));
    fs.writeFileSync(audioPath(root, audioKey(other)), Buffer.from("damaged"));
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    expect(snapshot).toMatchObject({ clips: 0, unavailable: 2 });
    store.deleteNote(card.id);
    restoreSnapshot(store, snapshot.id);
    expect(openStore(root).getNote(card.id)?.lemma).toBe("كِتَاب");
  });
});

describe("listing snapshots", () => {
  it("lists nothing when there is no snapshot folder, or a file in its place", () => {
    const { root } = workspace();
    expect(listSnapshots(root)).toEqual([]);
    fs.writeFileSync(path.join(root, "snapshots"), "in the way");
    expect(listSnapshots(root)).toEqual([]);
  });

  it("ignores folders that are not finished snapshots", () => {
    const { root, store } = workspace();
    store.createNote(kitab);
    const real = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    const base = path.join(root, "snapshots");
    fs.mkdirSync(path.join(base, ".partial-20261106T110000Z-manual"));
    fs.mkdirSync(path.join(base, "20261106T120000Z-manual"));
    fs.mkdirSync(path.join(base, "20261106T130000Z-manual"));
    fs.writeFileSync(path.join(base, "20261106T130000Z-manual", "manifest.json"), "{");
    fs.mkdirSync(path.join(base, "20261106T140000Z-manual"));
    fs.copyFileSync(path.join(base, real.id, "manifest.json"), path.join(base, "20261106T140000Z-manual", "manifest.json"));
    expect(listSnapshots(root).map((s) => s.id)).toEqual([real.id]);
  });

  it("lists the newest snapshot first", () => {
    const { root, store } = workspace();
    store.createNote(kitab);
    for (const time of ["10:00", "11:00", "09:00"]) createSnapshot(store, "manual", at(`2026-11-06T${time}:00Z`));
    expect(listSnapshots(root).map((s) => s.id)).toEqual([
      "20261106T110000Z-manual",
      "20261106T100000Z-manual",
      "20261106T090000Z-manual",
    ]);
  });
});

describe("snapshots made within the same second", () => {
  it("keeps both and lists the later one first", () => {
    const { root, store } = workspace();
    store.createNote(kitab);
    const first = createSnapshot(store, "manual", at("2026-11-06T14:03:22Z"));
    const second = createSnapshot(store, "manual", at("2026-11-06T14:03:22Z"));
    expect([first.id, second.id]).toEqual(["20261106T140322Z-manual", "20261106T140322Z-manual-2"]);
    expect(listSnapshots(root).map((s) => s.id)).toEqual([second.id, first.id]);
    restoreSnapshot(store, first.id, at("2026-11-06T15:00:00Z"));
    expect(openStore(root).allNotes()).toHaveLength(1);
  });

  it("leaves nothing behind when a snapshot cannot be taken", () => {
    const { root, store } = workspace();
    store.createNote(kitab);
    store.close();
    expect(() => createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"))).toThrow();
    const retry = createSnapshot(openStore(root), "manual", at("2026-11-06T10:00:00Z"));
    expect(retry.id).toBe("20261106T100000Z-manual");
  });
});

describe("restoring over damaged audio", () => {
  it("replaces a damaged clip in the audio folder with the snapshot's good copy", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    store.setAudio(card.id, "word", mp3, "tts");
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    fs.writeFileSync(audioPath(root, audioKey(mp3)), Buffer.from("damaged"));
    restoreSnapshot(store, snapshot.id);
    expect(fs.readFileSync(audioPath(root, audioKey(mp3)))).toEqual(mp3);
  });
});

describe("deleting a snapshot", () => {
  it("frees only its database copy and the clips that only it held", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    const form = card.forms[0].id;
    const twelve = Buffer.concat([mp3, Buffer.alloc(4)]);
    store.setAudio(card.id, form, mp3, "tts");
    const first = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    store.setAudio(card.id, "word", twelve, "tts");
    const later = createSnapshot(store, "manual", at("2026-11-06T11:00:00Z"));
    const before = snapshotUsage(root).totalBytes;
    deleteSnapshot(root, later.id);
    expect(before - snapshotUsage(root).totalBytes).toBe(later.dbBytes + 12);
    expect(listSnapshots(root).map((s) => s.id)).toEqual([first.id]);
    fs.rmSync(audioPath(root, audioKey(mp3)));
    restoreSnapshot(store, first.id);
    expect(fs.readFileSync(audioPath(root, audioKey(mp3)))).toEqual(mp3);
  });

  it("refuses an id that is not a snapshot", () => {
    const { root } = workspace();
    expect(() => deleteSnapshot(root, "../x")).toThrow("There is no such backup.");
    expect(() => deleteSnapshot(root, "20260101T000000Z-manual")).toThrow("There is no such backup.");
  });
});

describe("applying retention as snapshots are taken", () => {
  const limits = (keep: number, budgetBytes = Infinity) => ({ dailyKeep: keep, beforeKeep: keep, budgetBytes });

  it("keeps only the newest daily snapshots the count allows", () => {
    const { root, store } = workspace();
    store.createNote(kitab);
    for (const time of ["10:00", "11:00", "12:00"]) {
      createSnapshot(store, "daily", { now: new Date(`2026-11-06T${time}:00Z`), limits: limits(2) });
    }
    expect(listSnapshots(root).map((s) => s.id)).toEqual(["20261106T120000Z-daily", "20261106T110000Z-daily"]);
  });

  it("removes the oldest automatic snapshot that the byte budget cannot hold", () => {
    const { root, store } = workspace();
    store.createNote(kitab);
    const first = createSnapshot(store, "daily", { now: new Date("2026-11-06T10:00:00Z"), limits: limits(14) });
    const second = createSnapshot(store, "daily", {
      now: new Date("2026-11-06T11:00:00Z"),
      limits: limits(14, first.dbBytes + 5),
    });
    expect(listSnapshots(root).map((s) => s.id)).toEqual([second.id]);
  });

  it("never removes a manual snapshot, whatever the budget", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    store.setAudio(card.id, "word", mp3, "tts");
    const older = createSnapshot(store, "manual", at("2026-11-06T08:00:00Z"));
    const newer = createSnapshot(store, "manual", at("2026-11-06T09:00:00Z"));
    const daily = createSnapshot(store, "daily", { now: new Date("2026-11-06T10:00:00Z"), limits: limits(14, 0) });
    expect(listSnapshots(root).map((s) => s.id)).toEqual([daily.id, newer.id, older.id]);
  });

  it("reports what the snapshots use, counting manual ones only in the total", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    store.setAudio(card.id, "word", mp3, "tts");
    const manual = createSnapshot(store, "manual", at("2026-11-06T09:00:00Z"));
    expect(snapshotUsage(root)).toEqual({ automaticBytes: 0, totalBytes: manual.dbBytes + 8, budgetBytes: 1073741824 });
  });
});

describe("retention never gets in the way of a restore", () => {
  it("does not prune the snapshot it is restoring", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    const limits = { dailyKeep: 2, beforeKeep: 2, budgetBytes: Infinity };
    const oldest = createSnapshot(store, "before-reset", { now: new Date("2026-11-06T10:00:00Z"), limits });
    createSnapshot(store, "before-reset", { now: new Date("2026-11-06T11:00:00Z"), limits });
    store.deleteNote(card.id);
    restoreSnapshot(store, oldest.id, { now: new Date("2026-11-06T12:00:00Z"), limits });
    expect(openStore(root).getNote(card.id)?.lemma).toBe("كِتَاب");
  });

  it("still takes the snapshot when retention cannot run", () => {
    const { root, store } = workspace();
    store.createNote(kitab);
    fs.mkdirSync(path.join(root, "snapshots"));
    fs.writeFileSync(path.join(root, "snapshots", "audio"), "in the way");
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});
    const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    expect(listSnapshots(root).map((s) => s.id)).toEqual([snapshot.id]);
    expect(logged).toHaveBeenCalledTimes(1);
    logged.mockRestore();
  });
});

describe("deciding whether a daily snapshot is due", () => {
  const made = (reason: SnapshotReason, when: Date) => ({ reason, createdAt: when.toISOString() });
  const morning = new Date(2026, 10, 6, 8, 0);

  it("is not due while there are no cards", () => {
    expect(dailyDue([], new Date(2026, 10, 6, 9, 0), 0)).toBe(false);
  });

  it("is due when there is no daily snapshot yet", () => {
    expect(dailyDue([], new Date(2026, 10, 6, 9, 0), 1)).toBe(true);
  });

  it("is not due when a daily snapshot was made earlier the same day, and is due again after midnight", () => {
    expect(dailyDue([made("daily", morning)], new Date(2026, 10, 6, 23, 59), 1)).toBe(false);
    expect(dailyDue([made("daily", morning)], new Date(2026, 10, 7, 0, 5), 1)).toBe(true);
  });

  it("is not satisfied by a snapshot of another kind", () => {
    const others = [made("manual", morning), made("before-reset", morning)];
    expect(dailyDue(others, new Date(2026, 10, 6, 9, 0), 1)).toBe(true);
  });
});

async function zipOf(root: string, id: string): Promise<string> {
  const reader = backupZipStream(backupEntries(root, id)).getReader();
  const parts: Uint8Array[] = [];
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    parts.push(value);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-backup-zip-"));
  roots.push(dir);
  const file = path.join(dir, "backup.zip");
  fs.writeFileSync(file, Buffer.concat(parts));
  return file;
}

describe("importing a backup", () => {
  it("brings a backup into a fresh workspace as an imported snapshot that a restore turns into the cards, approvals and clips", async () => {
    const source = workspace();
    const card = source.store.createNote(kitab);
    source.store.setAudio(card.id, "word", mp3, "tts");
    expect(source.store.approve(card.id).ok).toBe(true);
    const made = createSnapshot(source.store, "manual", at("2026-11-06T10:00:00Z"));
    const zip = await zipOf(source.root, made.id);

    const fresh = workspace();
    const imported = importBackup(fresh.root, zip, at("2026-11-07T09:00:00Z"));
    expect(imported).toMatchObject({ reason: "imported", cards: 1, approved: 1, clips: 1, unavailable: 0 });
    restoreSnapshot(fresh.store, imported.id, at("2026-11-07T09:05:00Z"));
    const back = openStore(fresh.root);
    expect(back.getNote(card.id)).toMatchObject({ lemma: "كِتَاب", status: "approved" });
    expect(fs.readFileSync(audioPath(fresh.root, audioKey(mp3)))).toEqual(mp3);
  });
});

describe("refusing an import", () => {
  const refused = "This is not a Mufradat backup.";
  const newer = "This backup was written by a newer Mufradat.";
  const other = Buffer.concat([mp3, Buffer.alloc(4, 9)]);
  const entry = (key: string) => `audio/${key.slice(0, 2)}/${key}.mp3`;
  const now = at("2026-11-07T09:00:00Z");

  interface Pieces {
    manifest: Record<string, unknown>;
    db: Buffer;
  }

  async function pieces(): Promise<Pieces> {
    const source = workspace();
    const card = source.store.createNote(kitab);
    source.store.setAudio(card.id, "word", mp3, "tts");
    const made = createSnapshot(source.store, "manual", at("2026-11-06T10:00:00Z"));
    const dir = path.join(source.root, "snapshots", made.id);
    return {
      manifest: JSON.parse(fs.readFileSync(path.join(dir, "manifest.json"), "utf8")),
      db: fs.readFileSync(path.join(dir, "vocabulary.db")),
    };
  }

  async function zipFrom(contents: [string, Buffer | string][]): Promise<string> {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-crafted-"));
    roots.push(dir);
    const entries = contents.map(([name, bytes], i) => {
      const file = path.join(dir, `part-${i}`);
      fs.writeFileSync(file, bytes);
      return { name, file };
    });
    const reader = backupZipStream(entries).getReader();
    const parts: Uint8Array[] = [];
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      parts.push(value);
    }
    const zip = path.join(dir, "crafted.zip");
    fs.writeFileSync(zip, Buffer.concat(parts));
    return zip;
  }

  const json = (value: unknown) => JSON.stringify(value);
  const keyA = audioKey(mp3);
  const keyB = audioKey(other);

  const modified = (db: Buffer, change: (raw: Database.Database) => void) => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-modified-db-"));
    roots.push(dir);
    const file = path.join(dir, "v.db");
    fs.writeFileSync(file, db);
    const raw = new Database(file);
    change(raw);
    raw.close();
    return fs.readFileSync(file);
  };

  const bareDatabase = () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-bare-db-"));
    roots.push(dir);
    const file = path.join(dir, "bare.db");
    const raw = new Database(file);
    raw.exec("CREATE TABLE notes (id TEXT PRIMARY KEY, status TEXT); INSERT INTO notes VALUES ('a', 'approved')");
    raw.pragma("user_version = 1");
    raw.close();
    return fs.readFileSync(file);
  };

  const cases: [string, (p: Pieces) => Promise<string>, string][] = [
    ["a file that is not a zip", async () => zipFrom([]).then((zip) => (fs.writeFileSync(zip, "not a zip"), zip)), refused],
    ["a backup with no manifest", async (p) => zipFrom([["vocabulary.db", p.db], [entry(keyA), mp3]]), refused],
    ["a backup with no database", async (p) => zipFrom([["manifest.json", json(p.manifest)], [entry(keyA), mp3]]), refused],
    ["a manifest that is not one", async (p) => zipFrom([["manifest.json", "{"], ["vocabulary.db", p.db]]), refused],
    ["a manifest from a newer version", async (p) => zipFrom([["manifest.json", json({ ...p.manifest, schema: 99 })], ["vocabulary.db", p.db], [entry(keyA), mp3]]), newer],
    ["a database from a newer version", async (p) => zipFrom([["manifest.json", json(p.manifest)], ["vocabulary.db", modified(p.db, (raw) => raw.pragma("user_version = 99"))], [entry(keyA), mp3]]), newer],
    ["a damaged database", async (p) => zipFrom([["manifest.json", json(p.manifest)], ["vocabulary.db", "not a database"], [entry(keyA), mp3]]), "The backup's database is damaged."],
    [
      "a database with a trigger",
      async (p) =>
        zipFrom([
          ["manifest.json", json(p.manifest)],
          ["vocabulary.db", modified(p.db, (raw) => raw.exec("CREATE TRIGGER t AFTER UPDATE ON notes BEGIN SELECT 1; END"))],
          [entry(keyA), mp3],
        ]),
      "The backup's database is damaged.",
    ],
    [
      "a database in which notes is a view",
      async (p) =>
        zipFrom([
          ["manifest.json", json(p.manifest)],
          ["vocabulary.db", modified(p.db, (raw) => raw.exec("ALTER TABLE notes RENAME TO notes_real; CREATE VIEW notes AS SELECT * FROM notes_real"))],
          [entry(keyA), mp3],
        ]),
      "The backup's database is damaged.",
    ],
    [
      "a database that is not shaped like this app's",
      async (p) => zipFrom([["manifest.json", json(p.manifest)], ["vocabulary.db", bareDatabase()], [entry(keyA), mp3]]),
      "The backup's database is damaged.",
    ],
    [
      "a manifest that lists a clip twice",
      async (p) => zipFrom([["manifest.json", json({ ...p.manifest, audioKeys: [keyA, keyA] })], ["vocabulary.db", p.db], [entry(keyA), mp3]]),
      refused,
    ],
    ["a clip that is not what its name says", async (p) => zipFrom([["manifest.json", json(p.manifest)], ["vocabulary.db", p.db], [entry(keyA), other]]), refused],
    [
      "a second clip that is not what its name says, after a good one",
      async (p) =>
        zipFrom([["manifest.json", json({ ...p.manifest, audioKeys: [keyA, keyB] })], ["vocabulary.db", p.db], [entry(keyA), mp3], [entry(keyB), mp3]]),
      refused,
    ],
    ["a clip the manifest lists but the zip lacks", async (p) => zipFrom([["manifest.json", json(p.manifest)], ["vocabulary.db", p.db]]), refused],
    ["a clip the zip holds but the manifest does not list", async (p) => zipFrom([["manifest.json", json({ ...p.manifest, audioKeys: [] })], ["vocabulary.db", p.db], [entry(keyA), mp3]]), refused],
  ];

  it.each(cases)("refuses %s and leaves nothing behind", async (_name, build, message) => {
    const p = await pieces();
    const zip = await build(p);
    const fresh = workspace();
    expect(() => importBackup(fresh.root, zip, now)).toThrow(message);
    expect(listSnapshots(fresh.root)).toEqual([]);
    expect(snapshotUsage(fresh.root).totalBytes).toBe(0);
    const good = importBackup(fresh.root, await zipFrom([["manifest.json", json(p.manifest)], ["vocabulary.db", p.db], [entry(keyA), mp3]]), now);
    expect(good.id).toBe("20261107T090000Z-imported");
  });

  it("takes the card counts from the database, not from what the manifest claims", async () => {
    const p = await pieces();
    const zip = await zipFrom([["manifest.json", json({ ...p.manifest, cards: 999, approved: 999 })], ["vocabulary.db", p.db], [entry(keyA), mp3]]);
    const imported = importBackup(workspace().root, zip, now);
    expect(imported).toMatchObject({ cards: 1, approved: 0 });
  });
});

describe("a damaged clip in the snapshot folder", () => {
  it("is replaced from the live clip by the next snapshot, so that later snapshots still restore", () => {
    const { root, store } = workspace();
    const card = store.createNote(kitab);
    store.setAudio(card.id, "word", mp3, "tts");
    createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
    fs.writeFileSync(snapshotAudioPath(root, audioKey(mp3)), Buffer.alloc(0));
    const later = createSnapshot(store, "manual", at("2026-11-06T11:00:00Z"));
    store.deleteNote(card.id);
    fs.rmSync(path.join(root, "audio"), { recursive: true });
    restoreSnapshot(store, later.id, at("2026-11-06T12:00:00Z"));
    expect(fs.readFileSync(audioPath(root, audioKey(mp3)))).toEqual(mp3);
  });
});
