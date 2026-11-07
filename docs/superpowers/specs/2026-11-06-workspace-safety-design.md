# Workspace safety and long-term management: plan and design

Status: the owner has confirmed the seams, supplied measurements and chosen upload-based import. No question is open. Nothing here is implemented.

## How this plan is to be used

Implementation of every phase is guided by the `tdd` and `laws-of-simplicity` skills (in this environment `anthropic-skills:tdd` and `anthropic-skills:laws-of-simplicity`). Load both before each phase. How their rules apply here:

`tdd`
- Tests live only at the seams in "Seams", which the owner has confirmed. A new seam needs the owner's agreement first.
- Vertical slices: one failing test, the minimum code to pass it, then choose the next test from what that taught. The slice lists below are candidates in a likely order, not a batch to write up front.
- Tests observe behaviour through the public interface. A restore is checked by reading the card back through the store, not by opening the snapshot database; audio is checked by playing it, not by listing the backup folder.
- Expected values are literals (a byte count, a count of snapshots), never recomputed the way the code computes them.
- Mock only at boundaries: time (`vi.setSystemTime`) and nothing else. The file system is a real temporary directory, as in the existing tests. Limits such as the byte budget are passed in, so a test uses a budget of a few bytes instead of a gigabyte.
- Refactoring is not part of the red-green loop. It happens at the review stage, with `code-review`.

`laws-of-simplicity`
- Frame each phase with the ten laws before starting; run the ten-question checklist before review.
- After each change record a `simplicity_delta` from -5 to +5 in the commit message or PR description, and apply the keep/discard matrix: keep when the main goal improves and the delta is at least -2.
- Evidence before declaring problems: claims about the code are traced to the code and stated with the evidence; what is unchecked is written as an open question. "Evidence" below follows that.
- A failure gets a root-cause hypothesis written down before the next attempt; the same approach is not retried without a different hypothesis, and three failed attempts in one direction mean changing direction.

Repo conventions that apply throughout: no comments in code, never the `any` type, `pnpm typecheck` and `pnpm test` green before each commit, `pnpm e2e` for UI phases, README updated in the same commit as the behaviour it describes.

## What the workspace is today

| Part | What it holds | Replaceable? |
|---|---|---|
| `vocabulary.db` | Cards, forms, audio rows. One SQLite file, no schema version | No: hand-edited text and approvals |
| `audio/<xx>/<16 hex>.mp3` | Clips named by the hash of their bytes, written once, deleted only by Delete unused audio | Only by paying for generation again, or by hand for uploads |
| `exports/*.apkg` | Anki packages | Yes: the same cards always give the same bytes |
| `~/.config/mufradat/config.toml` | API keys | Outside the workspace; must never end up in a backup |

What cannot be undone today:

- Reset word calls `DELETE /api/notes/:id`, which deletes the card and cascades to its forms and audio rows.
- An edit overwrites in place; there is no history.
- Delete unused audio removes files for good.
- A database row can point at an audio file that is not there. Nothing checks for it, and Export then fails (see Evidence).

## What this plan is for

Confirmed by the owner:

- One person on one machine, a NixOS laptop.
- Mistakes that can be undone matter more than an audit trail.
- Automatic snapshots are silent; the list in the dialog is the only place they show.
- Snapshots must include the audio. Clips cost money to generate and must come back without regeneration.
- Automatic backups are limited both by count and by total size: 1 GiB preferred, 3 GiB at most and only if really needed. The count is left to this plan.

Success looks like: any mistake of the last weeks can be undone from the app in a few clicks, including the audio; a lost disk costs one file copy; and nothing about the existing flow (Continue, edit, approve, export) gets slower or longer.

## The options

| | A. Manual backup and import only | B. Version control of every change | C. Snapshots (chosen) |
|---|---|---|---|
| What it is | A "Back up" button that writes one file, and an import | Every edit recorded, browsable, restorable per card or per time | Whole-workspace snapshots, database and audio, taken automatically at the right moments and on demand, with one-file download and import |
| Undo a wrong Reset word | Only if a backup was taken beforehand | Yes, exactly | Yes: a snapshot is taken just before it |
| Undo a bad overwrite from last week | Only if remembered | Yes, exactly | Yes, to the daily snapshot; coarser than B |
| Survive disk loss | Yes, if the file was copied away | No, unless also exported | Yes, through Download |
| Needs the owner to remember | Yes | No | No for mistakes; Download is manual |
| New code | Small | Large: history storage, diffs, browsing UI, restore at a revision across notes, forms and audio, pruning | Medium |
| New surface to learn | Two buttons | A history browser | One dialog, one list |

Why not B. Fields save on blur, so a history of every change is mostly noise, and the cost sits in the interface that makes it usable (diffs, restoring forms and clips, pruning). Git in the workspace diffs a binary database poorly and shows git ideas to someone studying vocabulary. C gives the part of B that matters, undoing a recent mistake, at coarser grain for a fraction of the code. Restoring a single card from a snapshot is kept as an optional last phase.

Why not A alone. It protects nothing unless the owner remembers, and the dangerous moments (Reset word, a restore) are the ones nobody remembers.

## Design

### 1. Schema stamp

Set `PRAGMA user_version = 1` once the existing schema exists. Databases from before this have 0 and the same schema, and are stamped without touching their rows. `openStore` refuses a database with a higher version: "This workspace was written by a newer Mufradat." No migration mechanism is built until a second version exists.

### 2. Snapshots: `src/lib/snapshots.ts`

A new module, so the 290-line closure in `store.ts` does not grow.

    <workspace>/snapshots/
      audio/<xx>/<16 hex>.mp3        one copy of each clip any snapshot needs
      <id>/vocabulary.db
      <id>/manifest.json

- A snapshot is the database copy, the manifest, and every clip the manifest names being present in `snapshots/audio/`. It is built in a `.partial` folder and renamed into place, so an interrupted snapshot never appears in the list.
- The id is `YYYYMMDDTHHMMSSZ-<reason>`, so sorting by name is sorting by time. Reasons: `manual`, `daily`, `before-reset`, `before-restore`, `imported`.
- Manifest: `{ schema, createdAt, reason, cards, approved, audioKeys, unavailable }`.
- The database is copied with `VACUUM INTO`: synchronous, consistent and compact. The first test of Phase 1 settles whether the bundled SQLite supports it; the fallback is writing `db.serialize()` to the file, which is synchronous like `VACUUM INTO`.
- Audio is copied into the shared folder only when its key is not there yet, and every clip is hashed on the way in: the bytes must hash to the key in their name. A clip that is missing or does not match is skipped and counted in `unavailable`; the snapshot is still taken, because a daily snapshot that fails on one bad clip would protect nothing. The workspace check names those clips.
- Why a shared folder and not a copy per snapshot: clips never change, so one copy serves every snapshot, and the cost is about one extra copy of the audio instead of one per snapshot. A copy per snapshot would put fourteen times the audio on disk. Why a real copy and not a hard link: a link shares its bytes with the live file and behaves differently across file systems; the owner asked for the audio to be in the snapshot.
- Delete unused audio does not change. It deletes only from `audio/`, never from the snapshot folder, so a clip a backup holds survives it and comes back on restore.
- On disk, audio is therefore stored twice once snapshots exist: live, and in the snapshot folder. That is the price of an independent copy. It is counted in the budget and shown in the dialog.
- The snapshot folder lives inside the workspace, so copying the workspace folder with any ordinary tool copies the snapshots too.

Retention, two limits applied to automatic snapshots only (`daily`, `before-reset`, `before-restore`):

- Count: the newest 14 `daily` and the newest 14 `before-*` together. Fourteen daily snapshots cover fourteen days of use, which is where mistakes are noticed; the count is small because the byte budget is the real bound.
- Size: the automatic snapshots may use at most 1 GiB. Their size is the sum of their database copies plus every distinct clip they name, a shared clip counted once. When over a limit, the oldest automatic snapshot goes, then the size is worked out again, until both limits hold. The newest `daily` and the newest `before-*` are never removed for size: if they alone exceed the budget the dialog says so, and nothing is silently dropped.
- Clips held by no remaining snapshot are then removed from the snapshot folder.
- `manual` and `imported` snapshots are never removed automatically and do not count toward the budget. The dialog shows the total used by everything.
- The decision of what to remove is a pure function of snapshot sizes and limits, so its arithmetic is tested with small literal numbers.

When it runs: `context()` already runs on every request. It checks a cached date; when no `daily` exists for today and the workspace holds at least one card, it takes one. That snapshot is the state at the end of the previous session, which is what a mistake made today needs. This puts a side effect in a function tests call everywhere, so the decision is a pure function (`dailyDue`) tested on its own. The first snapshot ever copies all the audio and is synchronous; later ones copy only new clips.

Reset word: the `DELETE /api/notes/:id` handler takes a `before-reset` snapshot first, only when the card exists.

### 3. Restore

`POST /api/backups/<id>/restore`:

1. Check, changing nothing: the manifest parses, its schema is not newer than the app's, `PRAGMA integrity_check` on the snapshot database says `ok`, and every clip in `audioKeys` is in the snapshot folder and hashes to its key.
2. Take a `before-restore` snapshot of the current state.
3. Copy every clip the snapshot names that `audio/` lacks back from the snapshot folder, then close the store, copy the snapshot database beside `vocabulary.db`, rename it over, and drop the cached store so `context()` reopens it. The database is swapped last, so a failure while copying clips leaves the workspace as it was.

Step 2 always runs, so a restore is itself undone by restoring the `before-restore` snapshot.

### 4. One file out, one file in

- Download: `GET /api/backups/<id>` serves a zip of `manifest.json`, `vocabulary.db` and the snapshot's clips, built file by file from the snapshot folder with a streaming writer (data descriptors), not in memory. It never contains the config or the exports. The dialog shows when a snapshot was last downloaded, quietly and only as information: this is the one protection that survives losing the laptop, and automatic snapshots cannot give it.
- Import: the owner presses "Import backup…" in the dialog and picks the zip in the browser's file picker. The browser sends it to `POST /api/backups/import`. The route streams the body into a temporary file under `snapshots/`, refusing more than 3 GiB, checks it, turns it into an `imported` snapshot, and deletes the temporary file whether or not that worked. The same Restore button then switches to it, so there is one restore path, and the list row shows the manifest counts as the preview.
- Restoring from a file only matters after a disk loss or on a fresh install. Undoing a mistake on the same machine is the Restore button and involves no file.
- The 10 MiB problem and how it is handled. In the Next version in use (15.5.29), a request that passes through middleware has its body copied into memory and cut off at 10 MiB (see Evidence), and a backup passes that at around 300 words. The import route is therefore the one path left out of the middleware matcher, and it does the cross-site check itself. That check moves out of `middleware.ts` into one shared function, `refuseCrossSite(request)`, which the middleware and the import route both call. Every other route stays behind the middleware. The exclusion and the route's own check are each tested (Phase 3 slices 3, 5 and 7). If the source reading turns out wrong, the fallbacks in order are: raise `experimental.middlewareClientMaxBodySize` (every upload would then be held in memory, at most the size of a backup), or upload in chunks.
- The zip reader reads from the temporary file with positional reads, so memory stays flat. It accepts only what the writer produces: stored entries, no zip64, no encryption, no duplicates. Entry names must match an allowlist (`manifest.json`, `vocabulary.db`, `audio/xx/<16 hex>.mp3`) and are rebuilt from the matched groups, never used as paths. Each entry's CRC is checked, a clip's hash must equal the key in its name, and the database must pass `integrity_check` before the snapshot appears.
- Not merged: importing never combines two workspaces. That needs a rule for `UNIQUE (lemma, pos)` and for edits on both sides, and a single-machine owner has no use for it.

### 5. Workspace check

`GET /api/backups` returns the snapshot list, the sizes, and a check: card and approved counts, the `integrity_check` result, and the clips whose audio file is missing (note id, slot, key). The dialog shows the check only when something is wrong, with a link to each affected card, since regenerating a clip is already a card action. Export is changed to name the card and slot it cannot read.

### 6. Interface

One new quiet button, "Backups…", in the home footer beside Export and Delete unused audio, with the same dialog pattern as `UnusedAudio`. Inside:

- A line saying what the list is: "These backups are on this computer. Download one to keep a copy somewhere else." and, beside it, when one was last downloaded.
- Usage: what the automatic snapshots use of the 1 GiB, and the total, with a plain sentence when the newest snapshots alone exceed the budget.
- Back up now, and Import backup…, which opens the browser's file picker and shows that it is busy while the file is imported.
- The list, newest first: time, reason in words, "412 cards · 230 approved", and per row Download, Restore…, Delete.
- Restore asks once, showing that row's counts beside the current ones and saying the current state is saved first.
- No nagging, no notification for automatic snapshots, no settings.

Routes, in the existing `handle` / `fail` / `context` style: `GET` and `POST` on `/api/backups`, `POST` on `/api/backups/import`, `GET` and `DELETE` on `/api/backups/[id]`, `POST` on `/api/backups/[id]/restore`. Everything that changes anything is POST or DELETE, so the cross-site check covers it, in the middleware for every route but the import route and in the route itself for that one.

## How big is a snapshot

Measured on the owner's workspace, which has about 30 words: `audio/` is 976 KB and `vocabulary.db` is 88 KB. Everything below is arithmetic from those two numbers, so it is an estimate. The number of clips was not supplied, so the size of one clip is unknown, and all 30 cards are assumed to be voiced.

- Audio: 976 KB over 30 words is about 32 KB per word. If some of the 30 cards have no audio yet, the real figure per voiced card is higher, so the upper figures below use a margin of two (about 65 KB).
- Database: an empty schema already takes about 48 KB (twelve pages of 4 KB: the tables and their indexes), which leaves about 1.5 to 3 KB per card. Scaling 88 KB straight up gives the upper end.

| Words | Database copy | Audio (one shared copy) | One more daily snapshot adds | Everything the automatic snapshots hold, at most |
|---|---|---|---|---|
| 30 (today) | 88 KB | 1 MB | 88 KB, plus new clips | about 3.5 MB |
| 1,000 | 1.5 to 3 MB | 33 to 65 MB | 1.5 to 3 MB, plus new clips | about 75 to 150 MB |
| 5,000 | 8 to 15 MB | 160 to 325 MB | 8 to 15 MB, plus new clips | about 390 to 750 MB |

"At most" is the audio once plus 28 database copies, the full count of 14 daily and 14 `before-*`. In practice there are fewer.

"One shared copy" in plain terms: the clips of all snapshots live in one folder, one file per clip, and each snapshot only lists which clips it uses. A new snapshot copies just the clips that are not in the folder yet. At 1,000 words the first snapshot copies about 33 MB of audio; a day later, 20 new words add about 0.65 MB; so ten snapshots hold about 33 MB of audio plus ten small database copies, not ten times 33 MB.

- A snapshot contains the whole workspace, about 35 MB at 1,000 words and 170 to 340 MB at 5,000. Because clips are stored once, a new snapshot only costs a database copy plus the clips made since the last one.
- Audio is on disk twice once snapshots exist: live and in the snapshot folder.
- 1 GiB holds a finished 5,000-word deck even at the upper figures, which reach about 70% of it. 3 GiB is not needed, and the budget stays at 1 GiB.
- At the upper end the database copies are the biggest part after the audio. If space ever got tight, compressing the database copies (text compresses well) is the lever to pull before raising the limit. It is not built.
- Decision rule: if the usage the dialog shows for the automatic snapshots ever passes 0.8 GiB, raise the constant in `snapshots.ts`, to 3 GiB at most. It is a constant, not a setting.

## Seams

Confirmed by the owner on 2026-11-06, as the `tdd` skill requires before any test is written.

| # | Seam | What it catches | What it misses |
|---|---|---|---|
| 1 | `openStore`: schema version, the workspace check, unchanged unused-audio behaviour | Data-layer rules, legacy and newer workspaces | HTTP wiring, the interface |
| 2 | The snapshot module's public functions (create, list, restore, delete, `dailyDue`) driven with a real store in a temporary workspace, observed through the store and by playing clips | Snapshot, restore and retention behaviour end to end on disk | HTTP wiring, the cached store in `context()` |
| 3 | The retention decision (pure: snapshot sizes and limits in, what to remove out) | Count and byte-budget arithmetic exactly, shared clips counted once | Whether anything is actually deleted (seam 2) |
| 4 | Route handlers called as functions with the environment stubbed, as `audio/unused/route.test.ts` does: `/api/backups*`, `DELETE /api/notes/:id`, `/api/export` | Status codes, wiring, the daily snapshot through `context()`, the store reopening after a restore | A real browser, a real HTTP server |
| 5 | The zip writer and reader (file in, entries out) | Bad and hostile input, round trips | Everything outside the zip |
| 6 | Cypress against a production build | The dialog, a real download, layout at 1920px and 390px, hydration | Fine-grained logic; slow, so few specs |

## Phases

Each phase ships on its own and leaves the app working. Phases 0 and 1 give most of the protection. Slices are listed in the order they are likely to be taken, each one a single red-green cycle; the next is chosen after the last.

### Phase 0: stamp and check

1. (1) A workspace written by a newer version is refused with a clear message.
2. (1) A workspace from before this change opens and keeps its cards.
3. (1) The check reports a clip whose file is missing, with its note and slot, and reports a healthy workspace as healthy.
4. (4) Export with a missing clip answers with a message naming the card and the slot, not a path.

### Phase 1: snapshot engine, no interface

1. (2) Tracer bullet: take a snapshot, reset the card, restore, and the card is back with its forms. This first cycle settles `VACUUM INTO`.
2. (2) The card's clip plays after a restore, also when Delete unused audio ran in between.
3. (2) Restore saves the current state first; restoring that undoes the restore.
4. (2) Restore refuses and changes nothing for a newer schema, a damaged database, and a clip that is gone or does not match its hash.
5. (2) A snapshot taken while one clip's file is missing is still taken and reports one unavailable clip.
6. (3) The retention table with literal numbers: the 15th `daily` removes the oldest `daily` only; `before-*` has its own count; `manual` and `imported` are never removed; the byte budget removes oldest first; a clip shared by two snapshots counts once; the newest of each group stays even over budget.
7. (2) Applying retention removes the snapshots and the clips only they held, and keeps a clip another snapshot still holds; the reported usage drops by the literal number of bytes.
8. (2) `dailyDue`: one per date, none for an empty workspace, one again the next day.
9. (4) Reset word through the route takes a `before-reset` snapshot containing the card, and none for a card that does not exist.
10. (4) The first request of a day through `context()` takes the daily; a restore through the route leaves the app showing the restored cards.
11. (2) The list is newest first, ignores an unfinished snapshot, and Delete removes one.

### Phase 2: the Backups dialog

1. (6) Backups… opens a dialog listing snapshots with their counts; Back up now adds a row.
2. (6) Restore… asks once; Cancel changes nothing; Restore brings back a card that was reset.
3. (6) The dialog and the footer fit at 1920px and 390px; Escape closes the dialog and focus returns to its opener.
4. (6) The problem lines (missing clips with links to their cards, over budget) appear only when there is a problem.

README gets a Backups section and a corrected Files list. Moving the size and plural helpers out of `UnusedAudio.tsx` is a review-stage refactor.

### Phase 3: download and import

1. (5) The zip reader reads back what the writer writes, from a file.
2. (5) It rejects: a compressed entry, a duplicate name, a name such as `../x`, a wrong CRC, a truncated file, an unexpected entry, a clip whose hash is not its name.
3. (4) Download a snapshot, post it to the import route in a fresh workspace: the list shows an `imported` snapshot, and restoring it gives the cards, the approvals and playing audio. This is the "new laptop" case.
4. (4) A body that is garbage or truncated gets a 400, leaves the workspace untouched and leaves no temporary file behind.
5. (4) A request to the import route from another origin is refused by the route itself, and nothing is written.
6. (6) Import backup… with the file picker takes in a backup larger than 10 MiB whole, made by downloading a snapshot of a workspace with two 6 MiB clips (the app accepts one clip of at most 10 MiB), and shows it as `imported`. This proves the matcher exclusion.
7. (6) The running server refuses a cross-site request to the import route and to another route that changes data, so leaving one route out of the middleware dropped no other guard.
8. (6) Download starts a file.

If slice 6 fails, the fallbacks in "One file out, one file in" are tried in order.

### Phase 4: restore one card (decide after using 0 to 3)

Only if restoring a whole workspace to bring back one card has actually cost other work. Sketch: attach a snapshot read-only, copy that card's rows back by id, refuse with a clear message on a `(lemma, pos)` clash; its clips are already in the snapshot folder. It would sit near Reset word, the button beside the word counter at the top of a card.

## Not in this plan

Per-edit history, merging imported workspaces, syncing between machines, scheduled or cloud off-machine backups, encryption, backing up the config, a retention setting, notifications about backups, a migration framework.

## Laws of simplicity ledger

Scored at design time with the skill's scale (-5 to +5) and its keep rule: keep when the main goal improves and the delta is at least -2. The main goal is the owner's: undo mistakes, keep the audio, stay within the budget.

| Choice | Law | Delta | Verdict |
|---|---|---|---|
| Snapshots instead of per-edit history | Reduce, The One | -2: a new subsystem, but a fraction of B's | Keep |
| Audio in a shared snapshot folder | Differences, Trust | -2 | Keep |
| Audio only pinned by reference to live files | Reduce | 0 | Discard: fails "snapshots must include the audio" |
| A full copy of the audio per snapshot | Reduce | -3 | Discard: fourteen times the audio breaks the budget |
| Hard links instead of copies | Reduce | 0 | Discard: not an independent copy, differs by file system |
| One restore path; import adds a snapshot, then Restore | Organize, Learn | +2 | Keep |
| Upload import through its own route, left out of the middleware matcher, with the cross-site check in a shared function | Emotion, Trust | -2: one matcher exception and one shared function | Keep: the owner's choice, and it is the better experience |
| Import from a folder | Reduce | +2 | Discard: the owner judged the experience poor |
| Raise the body cap for the whole app | Reduce | -3: every upload held in memory | Discard, unless the matcher exception fails its test |
| Chunked upload | Reduce | -3 | Discard, unless both of the above fail |
| No settings; limits are constants | Reduce | +1 | Keep |
| A `before-restore` snapshot as the only undo | Trust, Emotion | 0 | Keep |
| Daily check inside `context()` | Time, Context | -1: a side effect in a shared function | Keep, with `dailyDue` pure and tested |
| Never silently dropping the newest snapshots | Trust, Failure | -1 | Keep: said in the dialog instead |
| "Last downloaded" line | Context | -1 | Keep, and removable: the one risk automatic snapshots cannot cover |
| No snapshot before Delete unused audio | Reduce | +1: unnecessary, since backups hold their own clips | Keep |

Checklist before each phase's review, from the skill: what can be removed; is the organisation clear; is time spent where it matters; can someone learn it quickly; is necessary complexity kept; does it fit the system; how does it feel to use; is it reliable; what did failures teach; does it serve what matters.

## Evidence

Traced in the code:

- No schema version: no `user_version` or migration code anywhere in `src`.
- Export with a missing clip: `POST /api/export` calls `buildApkg` with a reader that is `fs.readFileSync(audioPath(...))`; that throws inside `buildApkg`, and `handle` turns the error into a 500 carrying the raw message. `validate.ts`, `missing.ts` and `words.ts` never touch the file system, so nothing earlier catches it.
- Reset word: the route calls `store.deleteNote`, which is one `DELETE` that cascades.
- The cross-site guard in `src/middleware.ts` refuses only non-GET requests whose `Origin` differs; that is why everything that changes data in this plan is POST or DELETE.
- `zip.ts` can write a stored zip and cannot read one.
- Next 15.5.29, read from the package source: `experimental.middlewareClientMaxBodySize` defaults to 10485760 bytes. When middleware runs for a request, `cloneBodyStream` copies the body into two streams (`sandbox.js`) and ends both after that many bytes with a warning; afterwards `finalize` swaps the request's body for the buffered copy. So a request behind middleware is held in memory whole and cut at 10 MiB. `cloneBodyStream` is called only on those paths (middleware and edge-function execution, and proxying), and `getCloneableBody` itself only attaches end and error listeners, so a route outside the matcher should read the raw stream. That last step is inferred from the source, not run.
- The word list has 5,000 entries (`data/ranking.tsv`), which bounds the deck.
- `VACUUM INTO ?` with a bound parameter works with the bundled SQLite (the first snapshot test passes); the `serialize()` fallback is not needed.
- The day boundary is the local date and snapshot ids use UTC; the rule test pins this across midnight (00:05 the next day is due, 23:59 the same day is not).
- Snapshot speed, measured in the development container with 150 clips of 1 MB: the first snapshot (read, hash and write 150 MB) took 226 ms, a second with nothing new 8 ms, a third with one new clip 11 ms. That is about 1.5 ms per MB there, so a finished deck's 160 to 325 MB is well under a second; the owner's disk may be several times slower and will still be a few seconds at most, once. The first snapshot stays synchronous.
- Measured on the owner's workspace (about 30 words): `audio/` 976 KB, `vocabulary.db` 88 KB. The size table is derived from these.

Open, to be settled by the first slices:

- Whether a route left out of the middleware matcher really escapes the 10 MiB cap (Phase 3, slice 6).
- The average size of one clip: the clip count of the owner's workspace was not supplied, so the per-card audio figure assumes all 30 cards are voiced.
- Whether the daily check in `context()` is measurable on a real database.

## Questions

None open.

## Next step

The `writing-plans` skill turns each phase into task-sized steps; `tdd` and `laws-of-simplicity` guide those steps as described at the top.
