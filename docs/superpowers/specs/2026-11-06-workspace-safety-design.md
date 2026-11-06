# Workspace safety and long-term management: plan and design

Status: proposed, waiting for review. Nothing in this document is implemented.

## How this plan is to be used

Implementation of every phase below is to be guided by two skills, loaded before each phase starts:

- `tdd`: red, green, refactor. One failing test at a time, written through the public seams the repo already tests (the store, the route handlers, the Cypress specs), against real temporary directories as the existing tests do. The "Red" lists in each phase are the first tests to write, in order.
- `laws-of-simplicity`: every addition is checked against Maeda's laws before it is written. The ledger near the end records how the laws shaped this plan; extend it when a phase adds something.

When the plan was written, the Skill tool in that session reported both names as unknown, although both are enabled in the claude.ai skill library, so their text was not read. The ledger below is therefore written from the general laws, not from the skill. Whoever implements this loads both skills first; if either contradicts this plan, stop and raise it rather than quietly following one of the two. `superpowers:test-driven-development` is a usable stand-in for `tdd` if the library one still does not load.

Repo conventions that apply throughout: no comments in code, never the `any` type, `pnpm typecheck` and `pnpm test` green before each commit, `pnpm e2e` for UI phases, README updated in the same commit as the behaviour it describes.

## What the workspace is today

| Part | What it holds | Replaceable? |
|---|---|---|
| `vocabulary.db` | Cards, forms, audio rows. One SQLite file, default rollback journal, no schema version. | No: hand-edited text and approvals |
| `audio/<xx>/<16 hex>.mp3` | Clips named by the hash of their bytes, written once, never deleted except by Delete unused audio | Only by paying for generation again, or by hand for uploads |
| `exports/*.apkg` | Anki packages | Yes: the same cards always give the same bytes |
| `~/.config/mufradat/config.toml` | API keys | Outside the workspace; must never end up in a backup |

What cannot be undone today, read from the code:

- Reset word calls `DELETE /api/notes/:id`, which deletes the card and cascades to its forms and audio rows.
- An edit overwrites in place (`updateNote`); there is no history. Fields save on blur, so a "history of every change" would be one entry per blur.
- Delete unused audio removes files for good.
- There is no schema version (`CREATE TABLE IF NOT EXISTS` only), so a later schema change cannot tell an old database from a new one, and a backup from another app version cannot be checked.
- Export reads every clip with `fs.readFileSync` inside `buildApkg`. One audio row whose file is gone makes the whole export fail with a raw `ENOENT` message and a 500. Nothing tells the owner which card is at fault.
- `zip.ts` can write a stored zip but cannot read one.
- The cross-site guard in `src/middleware.ts` covers non-GET requests. Anything that restores or imports must therefore be POST, PUT or DELETE, never GET.

## What this plan is for

Said by the owner: make long-term management of the workspace easier; decide between full version control on every minor change, manual backups and imports, or something in between.

Assumed, to be confirmed in review:

- One person, mostly one machine, hundreds to low thousands of cards.
- The pain to remove is losing work: a wrong Reset word, a bad overwrite, a damaged file, a moved machine. It is not auditing who changed what.
- No new setting should have to be learned or maintained.

Success looks like: any mistake of the last weeks can be undone from the app in a few clicks; a disk loss or a move to another computer costs one file copy; and nothing about the existing flow (Continue, edit, approve, export) gets slower or longer.

## The options

| | A. Manual backup and import only | B. Version control of every change | C. Snapshots (recommended) |
|---|---|---|---|
| What it is | A "Back up" button that writes one file, and an import | Every edit recorded, browsable, restorable per card or per time | Whole-workspace snapshots taken automatically at the right moments and on demand, with one-file export and import |
| Undo a wrong Reset word | Only if a backup was taken beforehand | Yes, exactly | Yes: a snapshot is taken just before it |
| Undo a bad overwrite from last week | Only if remembered | Yes, exactly | Yes, to the daily snapshot; coarser than B |
| Survive disk loss or move machines | Yes | No, unless also exported | Yes, through Download and Import |
| Needs the owner to remember | Yes | No | No for mistakes; Download is manual |
| New code | Small | Large: history storage, diffs, browsing UI, restore-at-revision across notes, forms and audio rows, pruning | Medium |
| New surface to learn | Two buttons | A history browser | One dialog, one list |
| Touches existing code | Little | Every write path, or triggers plus a restore engine | Reset route, `context()`, the unused-audio check |

Why not B in full. Git in the workspace diffs a binary database poorly, needs git installed and shows git ideas to someone studying vocabulary. A log kept in SQLite triggers captures everything with little code but is useless without a restore interface, and that interface (per-field diffs, restore of forms and clips, pruning) is where the cost is. Because each field saves on blur, the log would be mostly noise. Most of what B gives, undoing a recent mistake, C gives at coarser grain for a fraction of the code.

Why not A alone. It protects against nothing unless the owner remembers, and the dangerous moments (Reset word, a restore, a version upgrade) are exactly the moments nobody remembers.

C, in one sentence: take a whole-database snapshot automatically once a day and just before anything destructive, keep a bounded number, make a snapshot the only thing you ever restore from, and let the same snapshot be downloaded as one file and imported back as a new snapshot.

The cheap, valuable part of B is restoring a single card from a snapshot. That is kept as an optional last phase, decided after living with the first three.

## Design

### 1. Schema stamp

Set `PRAGMA user_version = 1` after the existing schema is created. Databases created before this have 0 and hold exactly the same schema, so they are stamped without any change to their rows. `openStore` refuses a database whose version is higher than the app knows: "This workspace was written by a newer Mufradat." No migration mechanism is built until a second version exists.

### 2. Snapshots: `src/lib/snapshots.ts`

A new module, so the 290-line closure in `store.ts` does not grow.

- Layout: `<workspace>/snapshots/<id>/vocabulary.db` and `manifest.json`. The id is `YYYYMMDDTHHMMSSZ-<reason>`, so sorting by name is sorting by time; two in the same second get a numeric suffix.
- Reasons: `manual`, `daily`, `before-reset`, `before-restore`, `imported`.
- Manifest: `{ schema, createdAt, reason, cards, approved, audioKeys }`. It is the single source for the list rows, the restore preview and the audio pinning below.
- The database is copied with `VACUUM INTO`: synchronous, consistent, compact. Phase 1's first test proves the bundled SQLite supports it; the fallback is `db.backup()`.
- Audio is not copied. Clips are immutable and content-addressed, so a snapshot only needs to know which keys it needs.
- Pinning: `findUnusedAudio` treats keys named by any retained snapshot's manifest as in use. Delete unused audio therefore can never break a restore, and no snapshot is needed before it. The dialog says how many files are kept for backups, and they become unused again when their snapshots are pruned or deleted.
- Retention: `daily` keeps the newest 14; `before-reset` and `before-restore` together keep the newest 14; `manual` and `imported` are never pruned automatically. Two groups, one number, so a burst of resets cannot push out the dailies. Fourteen daily snapshots cover fourteen days of use, not fourteen calendar days.
- Daily: `context()` already runs on every request. It checks a cached date; when no `daily` exists for today and the workspace has at least one card, it takes one. The snapshot is therefore the state at the end of the previous session, which is what a mistake made today needs. This puts a side effect in a function tests call everywhere, so the decision is a pure function (`dailyDue(snapshots, today, cards)`) tested on its own, and tests control the date with `vi.setSystemTime`.
- Reset word: the `DELETE /api/notes/:id` handler takes a `before-reset` snapshot first, only when the card exists.

### 3. Restore

`POST /api/backups/<id>/restore`:

1. Validate without changing anything: manifest parses, schema not newer than the app, `PRAGMA integrity_check` on the snapshot database is `ok`, every audio key in the manifest exists in `audio/`.
2. Take a `before-restore` snapshot of the current state.
3. Close the store, copy the snapshot database beside `vocabulary.db`, rename it over, drop the cached store so `context()` reopens it.

A failure at step 1 changes nothing. Because step 2 always runs, a restore is itself undone by restoring the `before-restore` snapshot. That is the whole undo story.

### 4. One file in, one file out

- Download: `GET /api/backups/<id>` streams a zip of `manifest.json`, `vocabulary.db` and only the audio that snapshot needs. Config is never included, and neither are exports.
- Import: `PUT /api/backups` with the zip as the body, like the existing MP3 upload. Import does not replace anything: it adds the file to the list as an `imported` snapshot, and the same Restore button switches to it. One restore path, and the list row shows the manifest counts as the preview.
- The zip reader accepts only what the writer produces: stored entries, no zip64, no encryption, no duplicates. Entry names must match an allowlist (`manifest.json`, `vocabulary.db`, `audio/xx/<16 hex>.mp3`) and are rebuilt from the matched groups, never used as paths. Each entry's CRC is checked, an audio entry's hash must equal the key in its name, and the database must pass `integrity_check` before the snapshot appears.
- Not merged: importing never combines two workspaces. That needs a conflict rule for `UNIQUE (lemma, pos)` and for edits on both sides, and nobody has asked for it.

### 5. Workspace check

`GET /api/backups` returns the snapshot list and a check: card and approved counts, database and audio sizes, `integrity_check` result, and the audio rows whose file is missing (note id, slot, key). The dialog shows the check only when something is wrong, with a link to each affected card, since regenerating a clip is already a card action. Export is changed to name the card and slot it cannot read.

### 6. Interface

One new quiet button, "Backups…", in the home footer beside Export and Delete unused audio, using the same dialog pattern as `UnusedAudio`. Inside:

- Top line: the last backup, and a plain statement of what the list is: "These backups are on this computer. Download one to keep a copy somewhere else." Automatic snapshots protect against mistakes, not against losing the disk, and the dialog says so.
- Buttons: Back up now, Import backup…
- The list, newest first: time, reason in words, "412 cards · 230 approved", and per row Download, Restore…, Delete.
- Restore asks once, showing that row's counts beside the current ones and saying the current state is saved first.
- No nagging, no toast for automatic snapshots, no settings.

The routes, all in the existing `handle` / `fail` / `context` style: `GET` and `POST` and `PUT` on `/api/backups`, `GET` and `DELETE` on `/api/backups/[id]`, `POST` on `/api/backups/[id]/restore`.

## Phases

Each phase ships on its own and leaves the app working. Phases 0 and 1 give most of the protection.

### Phase 0: stamp and check

Red, in order:
1. A new workspace has `user_version` 1.
2. A database with `user_version` 0 and cards opens, is stamped, and loses nothing.
3. A database with `user_version` 99 makes `openStore` throw with the "newer Mufradat" message.
4. The check reports a missing audio file with its note and slot, the unused count, and `integrity_check` ok.
5. Export with a missing clip fails with a message naming the card and the slot, not a path.

### Phase 1: snapshot engine, no interface

Red, in order:
1. `VACUUM INTO` works on the store's database in a temporary directory.
2. A snapshot holds a database with the same rows, and a manifest with the right counts and audio keys.
3. Editing a card afterwards does not change the snapshot.
4. The list is newest first and ignores a directory with no manifest.
5. Retention: the fifteenth `daily` removes the oldest `daily` only; a burst of `before-reset` never removes a `daily`; `manual` is never removed.
6. Pinning: a clip removed or replaced after a snapshot keeps its file through Delete unused audio; once the snapshot is deleted the file is reported unused and deleted. This changes the sentence in README step 6 and in the dialog.
7. `dailyDue`: one `daily` per date, none for an empty workspace, one again the next day.
8. Reset word through the route takes a `before-reset` snapshot that contains the card, and none when the card does not exist.
9. Restore brings a reset card back with its forms and clips playing; the previous state is saved as `before-restore`; restoring that returns to the starting point; the store keeps working afterwards.
10. Restore refuses, and changes nothing, for a snapshot with a missing audio file, with a newer schema, or with a failing `integrity_check`.

### Phase 2: the Backups dialog

Red: a Cypress spec following `housekeeping.cy.ts`. Back up now adds a row; Cancel on Restore changes nothing; Restore brings back a card that was reset; the dialog and footer fit at 1920px and at 390px; Escape closes the dialog and focus returns to the opener. The size and plural helpers in `UnusedAudio.tsx` move to a shared module as the refactor step, with the existing spec as the safety net. README gets a Backups section and a corrected Files list.

### Phase 3: download and import

Red, in order:
1. The zip reader reads back what `zipStored` writes.
2. It rejects: a deflated entry, a duplicate name, a name such as `../x` or `audio/zz/…`, a wrong CRC, a truncated file, an unexpected entry.
3. An audio entry whose hash is not its name is rejected.
4. Garbage as the body gives a 400 and leaves the workspace untouched.
5. Import adds one `imported` snapshot and the audio it needs, and changes nothing else.
6. Two fresh workspaces: download from one, import and restore in the other, and the cards, approvals and playing audio match. This is the "new computer" scenario.
7. A cross-origin `PUT` to the import route is refused by the middleware.

Cypress: Download starts a file download; Import takes a file through `selectFile` and shows the new row.

### Phase 4: restore one card (decide after using 0 to 3)

Only if restoring a whole workspace to bring back one card has actually cost other work. Sketch: attach a snapshot read-only, copy that card's `notes`, `forms` and `audio` rows back by id, refuse with a clear message on a `(lemma, pos)` clash. Pinning already guarantees the clips exist. It would sit in Details beside Reset word.

## Not in this plan

Per-edit history, merging imported workspaces, syncing between machines, cloud or scheduled off-machine backups, encryption, backing up the config, a setting for retention, notifications about backups, and a migration framework.

## Laws of simplicity ledger

Written from the general laws; check against the `laws-of-simplicity` skill when implementing.

| Law | Where it shows |
|---|---|
| Reduce | No settings, no merge, no per-edit history, no snapshot of audio files, no snapshot before Delete unused audio (pinning makes it unnecessary) |
| Organize | One button, one dialog, one list; import and restore are the same list and the same verb |
| Time | Automatic snapshots cost the owner no time and no habit; Restore needs one confirmation |
| Learn | One new word, "backup"; a restore is undone with the same Restore button |
| Differences | Automatic and manual snapshots look different and are pruned differently; the dialog says what is on this computer and what a download is for |
| Context | The check appears only when something is wrong, with a link to the card; the restore preview is the row's own counts |
| Emotion | No nagging and no alarm; a restore says the current state is saved first |
| Trust | Validate before touching anything; hash and CRC checks on import; a failed restore changes nothing |
| Failure | Some risk cannot be simplified away (a lost disk), so the dialog states it and Download covers it |
| The One | Snapshots are the one idea; everything else is subtracted or reuses it |

## Risks and open points

- `VACUUM INTO` support in the bundled SQLite is expected but unverified; Phase 1 test 1 settles it.
- The Anki export builds its zip in memory, and a full backup including audio would too. Fine at hundreds of cards; measure on the real workspace and stream the zip if it passes a few hundred megabytes. Import reads the body into memory in the same way.
- Pinning means Delete unused audio frees less space until old snapshots age out. The dialog says so.
- Restore replaces `vocabulary.db` under a live connection: close first, then rename. The e2e spec covers reopening in a production build.
- The daily check runs inside `context()`, on the first request of a day. Time it on the real database; if it is noticeable, take it after the response.
- The day boundary is the local date; snapshot ids use UTC. A test pins this across midnight.
- Snapshots live inside the workspace, so copying the workspace folder copies them too. They are small.

## Questions for review

1. Is the assumption right that mistakes you can undo matter more than an audit trail? If not, B's history comes back into scope.
2. Is fourteen per group the right retention?
3. Silent automatic snapshots, with the list as the only place they show: agreed?
4. Do you ever work on more than one machine? If so, Phase 3 moves up and merging may deserve its own design.

## Next step

After review, the `writing-plans` skill turns each phase into task-sized steps; `tdd` and `laws-of-simplicity` guide those steps as described at the top.
