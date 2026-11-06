# Workspace Safety Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the workspace automatic and manual snapshots of the database and its audio, a restore that can itself be undone, a Backups dialog, and one-file download and upload, so any mistake of the last weeks can be undone and a lost disk costs one file copy.

**Architecture:** A new `snapshots.ts` takes a consistent database copy plus one shared, content-addressed copy of every clip any snapshot needs, and a pure `retention.ts` decides what to remove (14 daily, 14 `before-*`, 1 GiB). Restore validates, snapshots the current state, copies missing clips back and swaps the database last. Download streams a zip built from the snapshot folder; import is a dedicated route left out of the middleware matcher, with the cross-site check moved into a shared function.

**Tech Stack:** Next 15.5.29 route handlers, better-sqlite3 (`VACUUM INTO`), Node `fs`, `zlib.crc32` and streams, React dialog in the style of `UnusedAudio.tsx`, Vitest, Cypress.

**Spec:** `docs/superpowers/specs/2026-11-06-workspace-safety-design.md` (Phases 0 to 3; Phase 4 is decided after use and is not planned here).

**Skills:** load `tdd` and `laws-of-simplicity` (`anthropic-skills:tdd`, `anthropic-skills:laws-of-simplicity`) before each task. Tests go only at the six seams in the spec, one failing test at a time; results are observed through the public interface (the store, the routes, playing a clip), never by opening the snapshot folder; expected values are literals; mock only time; refactoring waits for Task 18. Each commit message ends with a `simplicity_delta` line (-5 to +5, with the reason).

**Before Task 1:** `CYPRESS_INSTALL_BINARY=0 pnpm install --frozen-lockfile`, then `pnpm typecheck && pnpm test` must be green. The repo requires Node 24 in `engines`; use it (the planning container had Node 22). Tasks 13 and 17 run `pnpm e2e`, which downloads the pinned Chrome.

## Global Constraints

- Node >=24, `pnpm@12.9.0`, Next 15.5.29, better-sqlite3 ^13.0.3.
- No comments in code. Never the `any` type. Temporary directories in tests are removed in `afterEach`.
- `pnpm typecheck` and `pnpm test` green before every commit; `pnpm e2e` for UI tasks; README updated in the commit that changes the behaviour.
- `PRAGMA user_version = 1`; a higher value is refused with "This workspace was written by a newer Mufradat."
- Layout: `<workspace>/snapshots/audio/<xx>/<16 hex>.mp3`, `<id>/vocabulary.db`, `<id>/manifest.json`, `.partial-<id>` while building. Id `YYYYMMDDTHHMMSSZ-<reason>`; a same-second collision appends `-2`, `-3` after the reason. Reasons: `manual`, `daily`, `before-reset`, `before-restore`, `imported`. Manifest: `{ schema, createdAt, reason, cards, approved, audioKeys, unavailable }`.
- A clip's key is the first 16 hex characters of the SHA-256 of its bytes (`audioKey`). Every clip copied into the snapshot folder is hashed; a missing or non-matching clip is skipped and counted in `unavailable`.
- Limits, constants in code and no setting: newest 14 `daily`; newest 14 `before-reset` and `before-restore` together; 1 GiB (`1024 ** 3`) for automatic snapshots, counting database copies plus distinct clips, a shared clip once. `manual` and `imported` are never removed automatically and not counted. The newest `daily` and the newest `before-*` are never removed for size.
- Restore: validate, then take `before-restore`, then copy missing clips back, then swap `vocabulary.db` last.
- Import: at most 3 GiB; the temporary file is always deleted. Zip entry names are only `manifest.json`, `vocabulary.db` and `audio/<xx>/<16 hex>.mp3`; stored entries only.
- Routes: `GET`/`POST /api/backups`; `POST /api/backups/import`; `GET`/`DELETE /api/backups/[id]`; `POST /api/backups/[id]/restore`. Everything that changes data is POST or DELETE. The middleware matcher leaves out only `/api/backups/import`.
- Error messages, exact: "There is no such backup.", "This backup was written by a newer Mufradat.", "The backup's database is damaged.", "The backup is missing or has damaged audio clips.", "This is not a Mufradat backup."
- Interface copy: button "Backups…"; line "These backups are on this computer. Download one to keep a copy somewhere else."; "Back up now"; "Import backup…"; Restore asks once and says the current state is saved first. Reuse the classes `action`, `action-main`, `action-quiet` and the dialog markup of `UnusedAudio.tsx`.
- The config and `exports/` are never part of a backup.

## Review Focus

Failure modes the spec implies and the tasks' main slices do not reach, most likely first. Each has its test in the task named.

1. The snapshot folder cannot be written (disk full, a file in its place): the daily snapshot must not fail the request (Task 11), and Reset word must refuse rather than delete without a safety net (Task 12).
2. Two snapshots in the same second, for example a double click on Back up now: both survive, newest first (Task 7).
3. The whole `audio/` folder is gone, not just some files, then a restore: every clip comes back (Task 5).
4. A backup or workspace from a newer version, which is what a NixOS generation rollback produces: refused with a clear message, nothing changed (Tasks 1, 6, 16).
5. A workspace with cards and no `audio/` folder at all: snapshot and restore work (Task 5).

## Where this plan corrects or adds to the spec

- The app caps one clip at 10 MiB (`MAX_AUDIO_BYTES`), so the spec's "11 MiB clip" in the import proof cannot exist. Task 17 uses two 6 MiB clips. The spec is patched with this plan.
- The `VACUUM INTO` fallback is `fs.writeFileSync(dest, db.serialize())`, synchronous like the primary, instead of the asynchronous `db.backup()`. The spec is patched.
- `context()` reopens a store that has been closed (`store.isOpen()`), so a restore needs no separate "drop the cached store" step (Task 10).
- Added decision: Reset word is refused when its `before-reset` snapshot cannot be made, with the message "The card was not reset because a backup could not be made first: <reason>" (Task 12). The owner should confirm this.
- Added: `refuseCrossSite` also refuses an `Origin` that does not parse, such as `null`; the old middleware threw on it (Task 16).
- Added test plumbing: `scripts/e2e.mjs` passes the workspace to Cypress as `CYPRESS_WORKSPACE`, and `cypress.config.mjs` gets a `removeAudio` task (Task 13).

## File Structure

Create:
- `src/lib/retention.ts` — pure `selectRemovals`, `DEFAULT_LIMITS`.
- `src/lib/snapshots.ts` — create, list, restore, delete, retention applied, usage, daily rule, import, last download.
- `src/lib/backup-zip.ts` — streaming zip writer and file-based reader for backups (`zip.ts` stays for Anki).
- `src/lib/origin.ts` — `refuseCrossSite`.
- `src/app/api/backups/route.ts`, `import/route.ts`, `[id]/route.ts`, `[id]/restore/route.ts` — the routes.
- `src/components/Backups.tsx` — the dialog.
- Tests: `src/lib/retention.test.ts`, `src/lib/snapshots.test.ts`, `src/lib/backup-zip.test.ts`, `src/app/api/backups/route.test.ts`, `src/app/api/backups/import/route.test.ts`, `src/app/api/backups/[id]/restore/route.test.ts`, `src/app/api/export/route.test.ts`, `cypress/e2e/backups.cy.ts`.

Modify: `src/lib/store.ts`, `src/lib/types.ts`, `src/lib/audio.ts`, `src/lib/anki.ts`, `src/lib/server.ts`, `src/middleware.ts`, `src/app/api/export/route.ts`, `src/app/api/notes/[id]/route.ts`, `src/app/page.tsx`, `scripts/e2e.mjs`, `cypress.config.mjs`, `README.md`.

---

## Phase 0: stamp and check

### Task 1: Schema version stamp

**Files:**
- Modify: `src/lib/store.ts` (`openStore`)
- Test: `src/lib/store.test.ts`

**Interfaces:**
- Produces: `export const SCHEMA_VERSION = 1` in `store.ts`; `openStore` throws `Error("This workspace was written by a newer Mufradat.")` after closing the handle when `user_version` exceeds it, and stamps a lower value.

- [ ] **Step 1: Write the failing test.** Add `describe("the workspace version")` to `store.test.ts` with a helper `const dir = () => fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-version-"))`:

```ts
it("refuses a workspace written by a newer version", () => {
  const root = dir();
  const raw = new Database(path.join(root, "vocabulary.db"));
  raw.pragma("user_version = 99");
  raw.close();
  expect(() => openStore(root)).toThrow("This workspace was written by a newer Mufradat.");
});

it("opens a workspace from before the version existed and keeps its cards", () => {
  const root = dir();
  const first = openStore(root);
  const card = first.createNote(kitab);
  first.close();
  const raw = new Database(path.join(root, "vocabulary.db"));
  raw.pragma("user_version = 0");
  raw.close();
  expect(openStore(root).getNote(card.id)?.lemma).toBe("كِتَاب");
});
```

- [ ] **Step 2: Run** `pnpm vitest run src/lib/store.test.ts -t "the workspace version"`. Expected: the first test FAILS (nothing thrown); the second already passes and stays as a guard.
- [ ] **Step 3: Implement** in `openStore`: read `db.pragma("user_version", { simple: true })` right after opening, close and throw above `SCHEMA_VERSION`, and after `db.exec(SCHEMA)` set `user_version` to `SCHEMA_VERSION` when lower.
- [ ] **Step 4: Run** the same command. Expected: both PASS.
- [ ] **Step 5: Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/store.ts src/lib/store.test.ts
git commit -m "Stamp the workspace schema version and refuse a newer one" -m "simplicity_delta: -1, one version stamp with no migration machinery"
```

### Task 2: Workspace check

**Files:**
- Modify: `src/lib/types.ts`, `src/lib/audio.ts`, `src/lib/store.ts`
- Test: `src/lib/store.test.ts`

**Interfaces:**
- Produces in `types.ts`: `interface MissingAudio { noteId: string; lemma: string; slot: Slot; slotName: string; key: string }` and `interface WorkspaceCheck { cards: number; approved: number; integrity: string; missingAudio: MissingAudio[] }`.
- Produces: `slotLabel(forms: Form[], slot: Slot): string` in `audio.ts` (`"word"`, `"example"`, or `form "<label>"`, falling back to the slot id); `Store.counts(): { cards: number; approved: number }`; `Store.check(): WorkspaceCheck`.

- [ ] **Step 1: Write the failing tests** in `store.test.ts`, `describe("checking the workspace")`, on `openStore(":memory:")` with the existing `kitab` and `mp3` fixtures:

```ts
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
  expect(s.check().missingAudio).toEqual([{ noteId: n.id, lemma: "كِتَاب", slot: "word", slotName: "word", key: audioKey(mp3) }]);
});
```

and a third test: a clip on `n.forms[0].id` whose file is gone has `slotName` `'form "pl."'`.
- [ ] **Step 2: Run** `pnpm vitest run src/lib/store.test.ts -t "checking the workspace"`. Expected: FAIL, `s.check is not a function`.
- [ ] **Step 3: Implement** `counts` (two `COUNT(*)` queries), `check` (`integrity_check` via `db.pragma("integrity_check", { simple: true })`; one join of `audio` and `notes` for rows with a key, keeping those whose `audioPath(root, key)` does not exist) and `slotLabel`. Add both to the object `openStore` returns.
- [ ] **Step 4: Run** the same command. Expected: PASS.
- [ ] **Step 5: Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/types.ts src/lib/audio.ts src/lib/store.ts src/lib/store.test.ts
git commit -m "Check the workspace for clips whose audio file is missing" -m "simplicity_delta: 0, one read-only method, nothing removed"
```

### Task 3: Export names the clip it cannot read

**Files:**
- Modify: `src/lib/audio.ts`, `src/lib/anki.ts` (`use` inside `buildApkg`), `src/app/api/export/route.ts`
- Test: `src/app/api/export/route.test.ts` (new)

**Interfaces:**
- Consumes: `slotLabel` (Task 2).
- Produces: `class MissingClipError extends Error` in `audio.ts`, constructed `(lemma: string, slotName: string)` with message `The audio for ${lemma} (${slotName}) is missing from the workspace. Generate or upload it again on the card.`; `buildApkg`'s `readAudio` becomes `(key: string, where: { lemma: string; slotName: string }) => Uint8Array` (existing callers with one parameter still fit).

- [ ] **Step 1: Write the failing test** in the style of `audio/unused/route.test.ts` (stub `XDG_CONFIG_HOME` and `MUFRADAT_WORKSPACE` to a temp dir):

```ts
it("names the card and the slot when a clip's file is missing", async () => {
  const { store } = context();
  const id = openCard(store, { rank: 10 })!;
  store.setAudio(id, "word", mp3, "tts");
  expect(store.approve(id).ok).toBe(true);
  fs.rmSync(audioPath(store.root, audioKey(mp3)));
  const res = await POST();
  expect(res.status).toBe(400);
  expect(await res.json()).toEqual({
    error: `The audio for ${store.getNote(id)!.lemma} (word) is missing from the workspace. Generate or upload it again on the card.`,
  });
});
```

- [ ] **Step 2: Run** `pnpm vitest run src/app/api/export/route.test.ts`. Expected: FAIL, status 500.
- [ ] **Step 3: Implement** `MissingClipError`; in `anki.ts` `use(slot)` passes `{ lemma: n.lemma, slotName: slotLabel(n.forms, slot) }` to `readAudio`; in the export route the reader throws `MissingClipError` when `audioPath` does not exist, and the handler turns that one error into `fail(e.message)`.
- [ ] **Step 4: Run** the same command and `pnpm vitest run src/lib/anki.test.ts`. Expected: PASS.
- [ ] **Step 5: Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/audio.ts src/lib/anki.ts src/app/api/export src/lib/anki.test.ts
git commit -m "Say which card's audio is missing when an export cannot read it" -m "simplicity_delta: +1, a raw ENOENT 500 becomes one plain sentence"
```

---

## Phase 1: snapshot engine

### Task 4: Retention decision

**Files:**
- Create: `src/lib/retention.ts`
- Modify: `src/lib/types.ts` (add `export type SnapshotReason = "manual" | "daily" | "before-reset" | "before-restore" | "imported"`)
- Test: `src/lib/retention.test.ts` (new)

**Interfaces:**
- Produces:

```ts
export interface RetentionEntry { id: string; reason: SnapshotReason; dbBytes: number; clips: Record<string, number> }
export interface Limits { dailyKeep: number; beforeKeep: number; budgetBytes: number }
export const DEFAULT_LIMITS: Limits
export function selectRemovals(entries: RetentionEntry[], limits: Limits): string[]
```

Entries are ordered by `id` ascending whatever order they arrive in; the result lists ids in removal order, oldest first. Automatic means `daily`, `before-reset`, `before-restore`; the groups are `daily` and `before`.

Take the slices in this order, each one red, then green. The helper is `const day = (id: string, dbBytes = 10, clips: Record<string, number> = {}): RetentionEntry => ({ id, reason: "daily", dbBytes, clips })`.

- [ ] **Slice 1, count.** Red: fifteen dailies `d01`..`d15` with `{ ...DEFAULT_LIMITS, budgetBytes: Infinity }` give `["d01"]`. Run `pnpm vitest run src/lib/retention.test.ts`; expected FAIL, `selectRemovals` not defined. Green: group by reason, keep the newest `dailyKeep` or `beforeKeep`.
- [ ] **Slice 2, groups.** Red: fourteen dailies plus fifteen alternating `before-reset` and `before-restore` give only the oldest `before-*` id. Green: `before-reset` and `before-restore` share one count.
- [ ] **Slice 3, never removed.** Red: thirty `manual` and `imported` entries with limits `{ dailyKeep: 1, beforeKeep: 1, budgetBytes: 1 }` give `[]`. Green: only automatic entries are candidates.
- [ ] **Slice 4, shared clips count once.** Red: `s1` holds clip A (100 bytes), `s2` holds A, `s3` holds B (50 bytes), each `dbBytes` 10, all daily: budget 175 gives `["s1"]` (A is still held by `s2`); budget 100 gives `["s1", "s2"]`. Green: drop the oldest removable until the sum of `dbBytes` plus distinct clip sizes fits, recomputing after each removal.
- [ ] **Slice 5, newest of each group stays.** Red: `d1` (10 bytes), `d2` (500), `r1` (500, `before-reset`) with budget 100 give `["d1"]`. Green: the newest entry of each group is never removable for size.
- [ ] **Slice 6, manual does not count.** Red: a `manual` entry holding a 1000-byte clip plus one daily of 10 bytes, budget 100, gives `[]`.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/retention.ts src/lib/retention.test.ts src/lib/types.ts
git commit -m "Decide which automatic snapshots to drop by count and by size" -m "simplicity_delta: +1, one pure function holds all the retention arithmetic"
```

### Task 5: Snapshot and restore, the tracer

**Files:**
- Create: `src/lib/snapshots.ts`, `src/lib/snapshots.test.ts`
- Modify: `src/lib/store.ts` (add `exportDatabase`, `audioKeys`), `src/lib/types.ts`

**Interfaces:**
- Produces in `types.ts`: `interface SnapshotInfo { id: string; reason: SnapshotReason; createdAt: string; schema: number; cards: number; approved: number; clips: number; unavailable: number; dbBytes: number }`.
- Produces in `store.ts`: `Store.exportDatabase(dest: string): void` (`db.prepare("VACUUM INTO ?").run(dest)`; if the bundled SQLite rejects it, write `db.serialize()` to `dest` with `fs.writeFileSync` instead) and `Store.audioKeys(): string[]` (distinct non-null keys).
- Produces in `snapshots.ts`:

```ts
export interface SnapshotOptions { now?: Date; limits?: Limits }
export function isSnapshotId(id: string): boolean
export function snapshotAudioPath(root: string, key: string): string
export function createSnapshot(store: Store, reason: SnapshotReason, options?: SnapshotOptions): SnapshotInfo
export function restoreSnapshot(store: Store, id: string, options?: SnapshotOptions): void
```

`snapshotAudioPath` is `audioPath(path.join(root, "snapshots"), key)`. `createSnapshot` builds in `.partial-<id>`, copies each referenced clip that is not yet in the snapshot folder after hashing it (skipping and counting unavailable ones), writes `manifest.json`, then renames the folder into place. `restoreSnapshot` closes the store, so callers open a new one with `openStore(root)`.

Test file setup: `mp3`, `kitab` as in `store.test.ts`; `workspace()` returns `{ root, store: openStore(root) }` on a fresh temp dir; `const at = (iso: string) => ({ now: new Date(iso) })`.

- [ ] **Slice 1, tracer: a reset card comes back.** Red:

```ts
it("brings a reset card back with its forms", () => {
  const { root, store } = workspace();
  const card = store.createNote(kitab);
  const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
  store.deleteNote(card.id);
  restoreSnapshot(store, snapshot.id);
  const back = openStore(root);
  expect(back.getNote(card.id)?.forms.map((f) => [f.label, f.arabic])).toEqual([["pl.", "كُتُب"]]);
});
```

Run `pnpm vitest run src/lib/snapshots.test.ts`; expected FAIL, module not found. Green: the minimal create and a restore that copies the snapshot database beside `vocabulary.db`, closes the store, and renames it over. This cycle settles whether `VACUUM INTO` works; if it throws, use the `serialize()` fallback.
- [ ] **Slice 2, the clip plays after Delete unused audio.** Red: the card has a word clip (`setAudio`); snapshot; `store.deleteNote`; `store.deleteUnusedAudio()`; restore; reopen; `fs.readFileSync(audioPath(root, back.getNote(card.id)!.audio.word.key!))` equals `mp3`. Green: `createSnapshot` hashes and copies each clip into the snapshot folder; `restoreSnapshot` copies back every clip the manifest names that `audio/` lacks.
- [ ] **Slice 3, the whole `audio/` folder is gone** (Review Focus 3). Red: same, with `fs.rmSync(path.join(root, "audio"), { recursive: true })` instead of Delete unused audio. Green: create `audio/<xx>/` on the way back.
- [ ] **Slice 4, no audio folder at all** (Review Focus 5). Red: a workspace with a card and no clips: `createSnapshot(...).clips` is `0` and a restore brings the card back. Green: tolerate the missing folder.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/snapshots.ts src/lib/snapshots.test.ts src/lib/store.ts src/lib/types.ts
git commit -m "Take snapshots of the database and audio and restore them" -m "simplicity_delta: -2, a new module, kept because the owner needs the audio in the snapshot"
```

### Task 6: Restore is safe and can be undone

**Files:**
- Modify: `src/lib/snapshots.ts`, `src/lib/snapshots.test.ts`

**Interfaces:**
- Consumes: Task 5's functions.
- Produces: `listSnapshots(root: string): SnapshotInfo[]` (newest first, by id); `restoreSnapshot` with its full behaviour. Before changing anything it throws, leaving the store open and the workspace untouched, with "There is no such backup." for a missing id or one failing `isSnapshotId` (so `../x` is refused), "This backup was written by a newer Mufradat." when the manifest `schema` exceeds `SCHEMA_VERSION`, "The backup's database is damaged." when opening the snapshot database read-only or its `integrity_check` fails, and "The backup is missing or has damaged audio clips." when a clip in `audioKeys` is absent from the snapshot folder or does not hash to its key. Then it takes a `before-restore` snapshot of the current state, copies clips back, and swaps the database last.

- [ ] **Slice 1, undoing a restore.** Red:

```ts
it("saves the current state first, and restoring that undoes the restore", () => {
  const { root, store } = workspace();
  const first = store.createNote(kitab);
  const snapshot = createSnapshot(store, "manual", at("2026-11-06T10:00:00Z"));
  const second = store.createNote({ ...kitab, lemma: "قَلَم", english: "pen", forms: [] });
  restoreSnapshot(store, snapshot.id, at("2026-11-06T11:00:00Z"));
  const afterRestore = openStore(root);
  expect(afterRestore.getNote(second.id)).toBeNull();
  const saved = listSnapshots(root).find((s) => s.reason === "before-restore")!;
  restoreSnapshot(afterRestore, saved.id, at("2026-11-06T12:00:00Z"));
  const undone = openStore(root);
  expect([first.id, second.id].map((id) => undone.getNote(id)?.lemma)).toEqual(["كِتَاب", "قَلَم"]);
});
```

Run `pnpm vitest run src/lib/snapshots.test.ts -t "undoes the restore"`; expected FAIL, `listSnapshots` not defined. Green: implement `listSnapshots` (directories not starting with `.` and not named `audio`, each with a readable manifest) and the `before-restore` step.
- [ ] **Slice 2, refusals** (Review Focus 4). Red: one `it.each` over the four tamperings, each asserting the exact message, that `store.getNote(card.id)` is still not null, and that `listSnapshots(root).length` is unchanged: manifest `schema` set to 99 (edit `manifest.json` with `fs`); `vocabulary.db` overwritten with `"not a database"`; a clip's snapshot file overwritten with `Buffer.from("x")` (path from `snapshotAudioPath`); the same file removed. Plus `restoreSnapshot(store, "../x")` throws "There is no such backup." Green: the validation step before anything else.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/snapshots.ts src/lib/snapshots.test.ts
git commit -m "Validate before a restore and save the current state first" -m "simplicity_delta: 0, the before-restore snapshot is the whole undo story"
```

### Task 7: Snapshots that are unfinished, damaged or simultaneous

**Files:**
- Modify: `src/lib/snapshots.ts`, `src/lib/snapshots.test.ts`

**Interfaces:**
- Consumes: Tasks 5 and 6. `listSnapshots` never throws: a missing `snapshots` folder, a file in its place, and folders with no manifest or an unparseable one yield nothing for those entries.
- Produces: unique ids; `SnapshotInfo.unavailable` filled.

- [ ] **Slice 1, unavailable clips.** Red: a card with two form clips; delete one clip file from `audio/` and overwrite the other with different bytes; `createSnapshot(...)` still returns, with `unavailable` equal to `2` and `clips` equal to `0`, and a restore still brings the card back. Green: skip and count missing or non-matching clips.
- [ ] **Slice 2, listing ignores what is not a snapshot.** Red: create `.partial-x`, a folder with no manifest, a folder whose `manifest.json` is `{`, next to one real snapshot: `listSnapshots(root).map((s) => s.id)` has only the real one; a `snapshots` that is a regular file gives `[]`. Green: guard each read.
- [ ] **Slice 3, newest first.** Red: three snapshots at `10:00`, `11:00`, `09:00` list as `11:00`, `10:00`, `09:00`. Green: sort by id descending.
- [ ] **Slice 4, same second** (Review Focus 2). Red: two `manual` snapshots at `2026-11-06T14:03:22Z` give ids `20261106T140322Z-manual-2` then `20261106T140322Z-manual` in the list, both restorable. Green: append `-2`, `-3` after the reason while the folder or its `.partial` exists.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/snapshots.ts src/lib/snapshots.test.ts
git commit -m "Keep snapshots from colliding and ignore unfinished ones" -m "simplicity_delta: 0, guards only, no new concept"
```

### Task 8: Usage, deleting and retention applied

**Files:**
- Modify: `src/lib/snapshots.ts`, `src/lib/types.ts` (add `interface SnapshotUsage { automaticBytes: number; totalBytes: number; budgetBytes: number }`), `src/lib/snapshots.test.ts`

**Interfaces:**
- Consumes: `selectRemovals`, `DEFAULT_LIMITS` (Task 4), `listSnapshots`.
- Produces: `deleteSnapshot(root: string, id: string): void` (removes the folder, then every clip named by no remaining manifest; throws "There is no such backup." for an unknown or malformed id); `applyRetention(root: string, limits?: Limits): string[]` (ids removed, then the same clip clean-up); `snapshotUsage(root: string, limits?: Limits): SnapshotUsage` (`totalBytes` is every database copy plus every file in the snapshot folder's `audio/`; `automaticBytes` follows the retention definition; `budgetBytes` is `limits.budgetBytes`). `createSnapshot` calls `applyRetention(root, options.limits)` last.

- [ ] **Slice 1, deleting frees only what only it held.** Red: a card with a form clip; `createSnapshot` at `10:00`; add a 12-byte clip on the word slot (`Buffer.concat([mp3, Buffer.alloc(4)])`); `second = createSnapshot` at `11:00`; `before = snapshotUsage(root).totalBytes`; `deleteSnapshot(root, second.id)`; `before - snapshotUsage(root).totalBytes` equals `second.dbBytes + 12`, and a restore of the first snapshot still plays the first clip. Green: the clean-up.
- [ ] **Slice 2, count applied.** Red: three `daily` snapshots with `limits: { dailyKeep: 2, beforeKeep: 2, budgetBytes: Infinity }` leave two listed, the oldest gone. Green: `applyRetention` and the call from `createSnapshot`.
- [ ] **Slice 3, budget applied.** Red: with `budgetBytes` set to the first snapshot's `dbBytes + 5`, a second `daily` leaves only the second listed. Green: reuse `selectRemovals` with `RetentionEntry` built from manifests and file sizes.
- [ ] **Slice 4, manual survives.** Red: a `manual` snapshot survives a `createSnapshot(..., { limits: { dailyKeep: 0, beforeKeep: 0, budgetBytes: 0 } })` of a `daily`, and `snapshotUsage` shows `automaticBytes` unaffected by it: with only the manual one present holding an 8-byte clip, `automaticBytes` is `0` and `totalBytes` is `dbBytes + 8`.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/snapshots.ts src/lib/types.ts src/lib/snapshots.test.ts
git commit -m "Prune snapshots by count and size and report what they use" -m "simplicity_delta: -1, one pass applies the pure decision"
```

### Task 9: Backups API: list, create, delete

**Files:**
- Create: `src/app/api/backups/route.ts`, `src/app/api/backups/[id]/route.ts`
- Test: `src/app/api/backups/route.test.ts` (new)

**Interfaces:**
- Consumes: `listSnapshots`, `snapshotUsage`, `createSnapshot`, `deleteSnapshot`, `Store.check()`.
- Produces: `GET /api/backups` returns `{ snapshots: SnapshotInfo[]; usage: SnapshotUsage; check: WorkspaceCheck }`; `POST /api/backups` returns `{ snapshot: SnapshotInfo }` for a `manual` snapshot; `DELETE /api/backups/[id]` answers 204, or 404 `{ error: "There is no such backup." }` when `listSnapshots` does not contain the id.

Test setup as in `audio/unused/route.test.ts` (stub the environment, temp workspace). Assert on snapshots filtered by `reason === "manual"`, because Task 11 adds daily ones.

- [ ] **Slice 1.** Red: an empty workspace gives `snapshots: []`, `usage.budgetBytes` `1073741824`, `check` `{ cards: 0, approved: 0, integrity: "ok", missingAudio: [] }`. Run `pnpm vitest run src/app/api/backups/route.test.ts`; expected FAIL, module not found. Green: the `GET` handler.
- [ ] **Slice 2.** Red: after `openCard(store, { rank: 10 })` and a `POST`, a `GET` lists one manual snapshot with `cards: 1`. Green: `POST`.
- [ ] **Slice 3.** Red: `DELETE` of that id answers 204 and the list is empty; `DELETE` of `20260101T000000Z-manual` and of `..` answers 404 and nothing is deleted. Green: the `[id]` route.
- [ ] **Slice 4.** Red: after removing a clip's file, `GET` shows it in `check.missingAudio`.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/app/api/backups
git commit -m "Add the routes that list, create and delete backups" -m "simplicity_delta: 0, thin handlers over the snapshot module"
```

### Task 10: Restore route and a store that reopens itself

**Files:**
- Create: `src/app/api/backups/[id]/restore/route.ts`
- Modify: `src/lib/store.ts` (add `isOpen(): boolean` as `db.open`), `src/lib/server.ts` (`context`)
- Test: `src/app/api/backups/[id]/restore/route.test.ts` (new)

**Interfaces:**
- Consumes: `restoreSnapshot`, `listSnapshots`.
- Produces: `POST /api/backups/[id]/restore` answers `{ ok: true }`, 404 "There is no such backup." for an unknown id, 400 with the thrown message for a refused one. `context()` opens a new store when the cached one is not `isOpen()`, and only closes the old one if it is still open.

- [ ] **Slice 1.** Red:

```ts
it("restores and leaves the app showing the restored cards", async () => {
  const { store } = context();
  const id = openCard(store, { rank: 10 })!;
  const { snapshot } = await (await createBackup()).json();
  store.deleteNote(id);
  const res = await restore(snapshot.id);
  expect(res.status).toBe(200);
  expect(context().store.getNote(id)).not.toBeNull();
});
```

Run `pnpm vitest run "src/app/api/backups/[id]/restore/route.test.ts"`; expected FAIL, module not found. Green: the route, `isOpen`, and the `context()` change.
- [ ] **Slice 2.** Red: an unknown id answers 404; a snapshot whose `manifest.json` has `schema` 99 answers 400 with "This backup was written by a newer Mufradat." and the card made after the snapshot is still there.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/app/api/backups src/lib/store.ts src/lib/server.ts
git commit -m "Restore through the API and let the store reopen itself" -m "simplicity_delta: +1, no separate cache-dropping step"
```

### Task 11: The daily snapshot

**Files:**
- Modify: `src/lib/snapshots.ts`, `src/lib/server.ts`
- Test: `src/lib/snapshots.test.ts`, `src/app/api/backups/route.test.ts`

**Interfaces:**
- Produces: `localDay(date: Date): string` (local `YYYY-MM-DD`); `dailyDue(snapshots: Pick<SnapshotInfo, "reason" | "createdAt">[], now: Date, cards: number): boolean` (true only with at least one card and no `daily` snapshot made on the local day of `now`).
- `context()` keeps, in its cached object, the last local day it dealt with. When `now`'s day differs and `store.counts().cards > 0`, it calls `createSnapshot(store, "daily")` if `dailyDue(listSnapshots(root), now, cards)`, catching any error with `console.error` and marking the day done either way. With no cards it marks nothing.

- [ ] **Slice 1, the rule.** Red, with dates built from `new Date(2026, 10, 6, ...)` so the test does not depend on the time zone: no cards gives `false`; no daily gives `true`; a daily at 08:00 and `now` 23:59 the same day gives `false`; `now` 00:05 the next day gives `true`; a `manual` or `before-reset` snapshot today does not count. Green: `localDay` and `dailyDue`.
- [ ] **Slice 2, through a request.** Red, in `route.test.ts` with `vi.useFakeTimers({ toFake: ["Date"] })` and `vi.setSystemTime(new Date(2026, 10, 6, 9, 0))`:

```ts
const dailies = async () =>
  (await (await GET()).json()).snapshots.filter((s: { reason: string }) => s.reason === "daily").length;
```

`dailies()` is `0` on an empty workspace; after `openCard(context().store, { rank: 10 })` it is `1`; again `1`; after `vi.setSystemTime(new Date(2026, 10, 7, 0, 5))` it is `2`. Green: the hook in `context()`.
- [ ] **Slice 3, an unwritable folder** (Review Focus 1). Red: write a regular file at `path.join(store.root, "snapshots")`, spy on `console.error` with an empty implementation, move to a new day: `expect(() => context()).not.toThrow()` and the spy was called once.
- [ ] **Measure** (settles an open question in the spec): in a scratchpad script, build a temporary workspace holding 150 clips of 1 MB and time the first `createSnapshot` and a second one. Record both figures in the spec's Evidence section. If the first takes more than about 2 seconds, report it to the owner before merging, because it runs inside the first request of a day.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/snapshots.ts src/lib/server.ts src/lib/snapshots.test.ts src/app/api/backups/route.test.ts
git commit -m "Take a daily snapshot on the first request of a day" -m "simplicity_delta: -1, a side effect in context(), kept small and tested"
```

### Task 12: Reset word backs up first

**Files:**
- Modify: `src/app/api/notes/[id]/route.ts` (`DELETE`), `README.md` (the Reset word sentence)
- Test: `src/app/api/notes/[id]/route.test.ts`

**Interfaces:**
- Consumes: `createSnapshot`, `listSnapshots`, `restoreSnapshot`.
- Produces: `DELETE` takes a `before-reset` snapshot when the card exists; if that throws it answers 500 `{ error: "The card was not reset because a backup could not be made first: <reason>" }` and deletes nothing. A missing card still answers 204 and takes no snapshot.

- [ ] **Slice 1.** Red: after `remove(id)` there is exactly one `before-reset` snapshot, and restoring it makes `context().store.getNote(id)` non-null; a second `remove(id)` and `remove("no-such-card")` leave the count at one. Run `pnpm vitest run "src/app/api/notes/[id]/route.test.ts"`; expected FAIL. Green: the snapshot before `deleteNote`.
- [ ] **Slice 2** (Review Focus 1). Red: with a regular file at `<workspace>/snapshots`, `remove(id)` answers 500 with a message containing "a backup could not be made first" and the card still exists.
- [ ] **README:** in step 2 of The flow, add that a backup is taken before Reset word.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add "src/app/api/notes/[id]" README.md
git commit -m "Take a backup before Reset word deletes a card" -m "simplicity_delta: +1, the riskiest button gets the safety net"
```

---

## Phase 2: the Backups dialog

### Task 13: Backups dialog, list, Back up now, Restore, Delete

**Files:**
- Create: `src/components/Backups.tsx`, `cypress/e2e/backups.cy.ts`
- Modify: `src/app/page.tsx` (add `<Backups />` after `<UnusedAudio />`), `scripts/e2e.mjs`, `cypress.config.mjs`, `README.md`

**Interfaces:**
- Consumes: the routes of Tasks 9 and 10; `api`, `post` from `src/components/api.ts`.
- Produces: a client component with the opener "Backups…", a modal titled "Backups", an `ol[aria-label="Backups"]` whose rows read `<reason in words> · <n card(s)> · <m approved>` with buttons "Restore…" and "Delete", a "Back up now" button, the line from the Global Constraints, a usage line (automatic use of the 1 GiB, and the total), and problem lines (each missing clip with a link to `/cards/<id>`; a sentence when `automaticBytes` exceeds `budgetBytes`). Restore asks once, shows the row's counts beside the current ones and says the current state is saved first; all buttons are disabled while busy. Reason words: manual "Backed up by you", daily "Daily", before-reset "Before a reset", before-restore "Before a restore", imported "Imported". The over-budget sentence has no e2e test (the budget cannot be reached from the browser); review it by eye.
- Produces for tests: `CYPRESS_WORKSPACE` (the e2e workspace path) in the Cypress environment, read as `config.env.workspace`, and a task `removeAudio(key: string): null` deleting `audio/<xx>/<key>.mp3` there.

Cypress selectors used below: `const opener = 'button:contains("Backups")'`, `const dialog = "dialog[open]"`, `const rows = 'dialog[open] ol[aria-label="Backups"] > li'`.

- [ ] **Step 1: infrastructure.** In `scripts/e2e.mjs` pass `env: { ...process.env, CYPRESS_WORKSPACE: env.MUFRADAT_WORKSPACE }` to the Cypress `spawn`; in `cypress.config.mjs` add `setupNodeEvents(on, config)` registering the `removeAudio` task and returning `config`.
- [ ] **Slice 1.** Red:

```ts
it("adds a row when Back up now is pressed", () => {
  cy.visit("/");
  cy.hydrated(opener);
  cy.get(opener).click();
  cy.get(rows).its("length").then((before) => {
    cy.contains(dialog + " button", "Back up now").click();
    cy.get(rows).should("have.length", before + 1);
  });
});
```

Run `pnpm e2e --spec cypress/e2e/backups.cy.ts`; expected FAIL, no such button. Green: the component and the page footer.
- [ ] **Slice 2, restore.** Red: visit `/open?rank=411`, take the card id from `location.pathname`, press Back up now, `cy.request("DELETE", "/api/notes/" + id)`, reload, open the dialog, press Restore… on the row that says "Backed up by you", press Cancel and assert `cy.request({ url: "/cards/" + id, failOnStatusCode: false }).its("status")` is `404`; press Restore… then Restore and assert it is `200`. Green: the confirm step.
- [ ] **Slice 3, layout.** Red: at `1920×1100` and `390×844` the opener and the dialog stay inside the viewport and the page has no horizontal scroll (copy the checks of `housekeeping.cy.ts`); Escape closes the dialog and the opener has focus again.
- [ ] **Slice 4, problem lines.** Red: on a card for rank 412 `PUT /api/audio/<id>/word` a clip (the response's `note.audio.word.key` is the key), `cy.task("removeAudio", key)`, open the dialog: it contains a link to `/cards/<id>`; after `DELETE /api/audio/<id>/word` and reopening, no problem line is shown.
- [ ] **README:** add a "Backups" section (what is taken and when, the 14 and 14 and 1 GiB limits, that audio is included, Restore and what it does first) and add `snapshots/` to the workspace list under Files.
- [ ] **Commit** after `pnpm typecheck && pnpm test && pnpm e2e --spec cypress/e2e/backups.cy.ts`:

```bash
git add src/components/Backups.tsx src/app/page.tsx cypress scripts/e2e.mjs cypress.config.mjs README.md
git commit -m "Add the Backups dialog with Back up now and Restore" -m "simplicity_delta: -1, one dialog, one list, no settings"
```

---

## Phase 3: download and import

### Task 14: Backup zip writer and reader

**Files:**
- Create: `src/lib/backup-zip.ts`, `src/lib/backup-zip.test.ts`

**Interfaces:**
- Produces:

```ts
export interface BackupEntry { name: string; file: string }
export function backupZipStream(entries: BackupEntry[]): ReadableStream<Uint8Array>
export interface BackupZip { names: string[]; read(name: string): Buffer; copyTo(name: string, dest: string): void; close(): void }
export function openBackupZip(file: string): BackupZip
```

The writer reads each file in chunks, keeps a running `crc32(chunk, crc)` from `node:zlib`, writes stored local headers with flag bit 3 and a data descriptor after each entry, and the central directory last; nothing is held whole in memory. The reader finds the end-of-central-directory record in the last 64 KiB and parses the central directory. Every failure throws `Error("This is not a Mufradat backup.")`: a method other than stored, an encryption flag, zip64 markers, a duplicate name, a name outside `^(manifest\.json|vocabulary\.db|audio/[0-9a-f]{2}/[0-9a-f]{16}\.mp3)$` or whose folder is not the first two hex characters of the key, a CRC mismatch, truncation, or a non-database entry over 10 MiB read through `read`. `copyTo` streams `vocabulary.db` to disk with the CRC check.

Test helper: write `backupZipStream(...)` to a temp file with `pipeline(Readable.fromWeb(stream), fs.createWriteStream(file))`; hostile inputs are built with the existing `zipStored` from `src/lib/zip.ts` and edited bytes.

- [ ] **Slice 1, round trip.** Red: entries `manifest.json`, `vocabulary.db`, `audio/ab/abcdef0123456789.mp3` written then read give the same names (sorted) and the same bytes. Run `pnpm vitest run src/lib/backup-zip.test.ts`; expected FAIL, module not found. Green: writer and reader.
- [ ] **Slice 2, rejections.** Red, one `it.each` over: a compressed entry (method field patched to 8), a duplicate name, `../x`, an unexpected `readme.txt`, a clip whose folder does not match its key, a flipped data byte, a truncated file, a `manifest.json`-named entry over 10 MiB read with `read`; each throws "This is not a Mufradat backup." Green: the checks.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/backup-zip.ts src/lib/backup-zip.test.ts
git commit -m "Write backup zips as a stream and read them back safely" -m "simplicity_delta: -2, a second zip path, kept because backups are too big to hold in memory"
```

### Task 15: Download a snapshot

**Files:**
- Modify: `src/app/api/backups/[id]/route.ts` (add `GET`), `src/app/api/backups/route.ts` (`GET` adds `lastDownloadedAt`), `src/lib/snapshots.ts`
- Test: `src/app/api/backups/route.test.ts`

**Interfaces:**
- Consumes: `backupZipStream`, `openBackupZip` (Task 14), `snapshotAudioPath`.
- Produces in `snapshots.ts`: `backupEntries(root: string, id: string): BackupEntry[]` (`manifest.json`, `vocabulary.db` and `audio/<xx>/<key>.mp3` for each key in the manifest); `recordDownload(root: string, now?: Date): void` and `lastDownload(root: string): string | null` (ISO text in `snapshots/last-download`). `GET /api/backups/[id]` answers a zip with `Content-Type: application/zip` and `Content-Disposition: attachment; filename="mufradat-backup-<id>.zip"`, or 404 "There is no such backup."; `GET /api/backups` gains `lastDownloadedAt: string | null`.

- [ ] **Slice 1.** Red: a snapshot with one clip downloaded through the route, written to a temp file and opened with `openBackupZip`, has the names `manifest.json`, `vocabulary.db` and `audio/<xx>/<key>.mp3`, and `JSON.parse(read("manifest.json").toString()).cards` is `1`. Run `pnpm vitest run src/app/api/backups/route.test.ts -t "download"`; expected FAIL, the handler is not exported. Green: `backupEntries` and `GET`.
- [ ] **Slice 2.** Red: with `vi.setSystemTime(new Date("2026-11-06T10:00:00Z"))`, `GET /api/backups` has `lastDownloadedAt: null` before a download and `"2026-11-06T10:00:00.000Z"` after; an unknown id answers 404. Green: `recordDownload`, `lastDownload`.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/app/api/backups src/lib/snapshots.ts
git commit -m "Download a snapshot as one zip and remember when" -m "simplicity_delta: 0, one route and one timestamp file"
```

### Task 16: Import a backup

**Files:**
- Create: `src/lib/origin.ts`, `src/app/api/backups/import/route.ts`
- Modify: `src/lib/snapshots.ts`, `src/middleware.ts`
- Test: `src/lib/snapshots.test.ts`, `src/app/api/backups/import/route.test.ts` (new)

**Interfaces:**
- Consumes: `openBackupZip`, `SCHEMA_VERSION`, `audioKey`.
- Produces: `importBackup(root: string, file: string, options?: SnapshotOptions): SnapshotInfo` in `snapshots.ts`: opens the zip, requires `manifest.json` and `vocabulary.db`, refuses a manifest `schema` above `SCHEMA_VERSION` ("This backup was written by a newer Mufradat."), copies the database out and requires `integrity_check` ok ("The backup's database is damaged."), requires the `audio/` entries to be exactly the manifest's `audioKeys` and each to hash to its key (else "This is not a Mufradat backup."), then builds an `imported` snapshot (manifest `reason` and `createdAt` rewritten, clips added to the snapshot folder) and returns its info. Nothing is left behind on failure.
- Produces: `refuseCrossSite(req: Request): Response | null` in `origin.ts`: for a method other than GET, a 403 `{ error: "Cross-site request refused." }` when `Origin` is present and its host differs from the `Host` header or does not parse; otherwise `null`. `middleware` becomes `refuseCrossSite(req) ?? NextResponse.next()` with `config.matcher = ["/api/((?!backups/import$).*)"]`.
- Produces: `POST /api/backups/import`: runs `refuseCrossSite`; answers 413 "That file is larger than 3 GiB." past 3 GiB (checked on `Content-Length` and while streaming); streams `req.body` into `snapshots/.incoming-<random>.zip`; calls `importBackup`; always removes the temporary file; answers `{ snapshot: SnapshotInfo }`, or 400 with the error message.

Route test setup: a workspace A is stubbed through `MUFRADAT_WORKSPACE`, a snapshot downloaded through the Task 15 route, then the variable is stubbed to a fresh workspace B (`context()` reopens) before importing.

- [ ] **Slice 1, the new-laptop case.** Red: the zip bytes of a snapshot of A with a card and a clip, posted to the import route in B (`new Request(url, { method: "POST", body: zip })`), answer 200 with `reason: "imported"`; restoring that snapshot through the restore route gives B the card, its approval state and a clip whose bytes equal the original. Run `pnpm vitest run src/app/api/backups/import/route.test.ts`; expected FAIL, module not found. Green: `importBackup` and the route.
- [ ] **Slice 2, bad bodies.** Red: a body of `"not a zip"` and the first half of a valid zip each answer 400 "This is not a Mufradat backup."; afterwards `listSnapshots` is empty and `snapshots/` holds no `.incoming-*` file.
- [ ] **Slice 3, newer version** (Review Focus 4). Red: a zip built with `backupZipStream` from a manifest edited to `schema` 99 answers 400 "This backup was written by a newer Mufradat." and adds nothing.
- [ ] **Slice 4, cross-site.** Red: with the header `Origin: http://evil.example` and `Host: 127.0.0.1` the route answers 403 and writes nothing; so does `Origin: null`. Green: `refuseCrossSite`, called first in the route.
- [ ] **Slice 5, the size cap.** Red: a request whose `content-length` header is `3 * 1024 ** 3 + 1` answers 413 "That file is larger than 3 GiB." and writes nothing. If the runtime will not let a `Request` carry that header, test the byte-counting step instead by giving the route's streaming helper a small limit as an argument.
- [ ] **Slice 6, wiring.** Change `src/middleware.ts` to use `refuseCrossSite` and the new matcher, then run `pnpm build`. Expected: the build succeeds and lists the middleware. Task 17's Cypress slices prove the exclusion on a real server.
- [ ] **Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/lib/origin.ts src/lib/snapshots.ts src/middleware.ts src/app/api/backups/import src/lib/snapshots.test.ts
git commit -m "Import a backup through its own route with a shared cross-site check" -m "simplicity_delta: -2, one matcher exception and one shared function, the owner's choice"
```

### Task 17: Download and import in the dialog, and the real-server proof

**Files:**
- Modify: `src/components/Backups.tsx`, `cypress/e2e/backups.cy.ts`, `README.md`

**Interfaces:**
- Consumes: Tasks 15 and 16.
- Produces: per row a "Download" link (`href="/api/backups/<id>"`, `download`); a quiet "Last downloaded: never" or the local time of `lastDownloadedAt` beside the explanatory line; an "Import backup…" button that opens the picker of a hidden `input type="file"` (`accept=".zip,application/zip"`), posts the file as the body of `POST /api/backups/import`, shows "Importing…" with `role="status"` while busy and any error with `role="alert"`, then reloads the list.

- [ ] **Slice 1, the proof (spec Phase 3, slice 6).** Red: on a card for rank 421, `PUT` two different 6 MiB clips to the word and example slots (each starts with the bytes `ff fb 90 00`, then a distinguishing byte, then padding), press Back up now, find the new manual snapshot in `cy.request("/api/backups")`, download it with `cy.request({ url: "/api/backups/" + id, encoding: "binary" })` and assert the body is over `10 * 1024 * 1024` bytes, then `cy.get('input[type="file"]').selectFile({ contents: Cypress.Buffer.from(body, "binary"), fileName: "backup.zip" }, { force: true })`. The dialog gains an "Imported" row, and the imported snapshot's `clips` equals the source's. Delete both snapshots through the API afterwards. Run `pnpm e2e --spec cypress/e2e/backups.cy.ts`; expected FAIL, no file input. Green: the Import button. If the upload arrives truncated, the cap applies to this route: apply the fallbacks in the spec's order (raise `experimental.middlewareClientMaxBodySize`, then chunked upload) and record the failed hypothesis first.
- [ ] **Slice 2, cross-site on a real server.** Red: `cy.request({ method: "POST", url: "/api/backups/import", headers: { Origin: "http://evil.example" }, body: "x", failOnStatusCode: false })` and the same to `/api/backups` both answer `403`. This proves leaving one route out of the middleware dropped no other guard.
- [ ] **Slice 3, download.** Red: pressing a row's Download link leaves `mufradat-backup-<id>.zip` in `Cypress.config("downloadsFolder")` (`cy.readFile(path, null).should("exist")`), and the dialog's last-downloaded text is no longer "never".
- [ ] **README:** describe Download and Import backup…, and that Delete unused audio never touches a backup's own copies.
- [ ] **Commit** after `pnpm typecheck && pnpm test && pnpm e2e --spec cypress/e2e/backups.cy.ts`:

```bash
git add src/components/Backups.tsx cypress/e2e/backups.cy.ts README.md
git commit -m "Download and import backups from the dialog" -m "simplicity_delta: -1, two buttons on one list"
```

### Task 18: Review-stage cleanup

**Files:**
- Modify: `src/components/UnusedAudio.tsx`, `src/components/Backups.tsx`; Create: `src/components/format.ts`

**Interfaces:**
- Produces: `size(bytes: number): string` and `noun(count: number): string` exported from `src/components/format.ts`, moved unchanged from `UnusedAudio.tsx` and used by both components.

- [ ] **Step 1:** run the `laws-of-simplicity` ten-question checklist over the whole branch and record a final `simplicity_delta` per phase in the PR description; run the `code-review` skill.
- [ ] **Step 2:** move the helpers, keeping `cypress/e2e/housekeeping.cy.ts` as the safety net (`pnpm e2e --spec cypress/e2e/housekeeping.cy.ts`; expected PASS unchanged).
- [ ] **Step 3: Commit** after `pnpm typecheck && pnpm test`:

```bash
git add src/components
git commit -m "Share the size and plural helpers between the two dialogs" -m "simplicity_delta: +2, one copy of each helper"
```
