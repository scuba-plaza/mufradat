# Plan: Suspend word

A suspended word is parked: it is never exported, it never shows up in any search
except one that asks for `status:suspended`, and the card editor's previous/next
navigation never lands on it unless the search that produced the queue asked for
`status:suspended`.

## Agreed behaviour

- **UI**: Suspend/Unsuspend lives in the **card editor only** (near Reset word / in
  the ⋯ menu area). No list-row action, no counter anywhere; the only way to find
  suspended words is searching `status:suspended`.
- **Approval**: suspending a card **takes its approval back** (like removing audio
  does). Unsuspending does not re-approve; the card returns to `progress`/`ready`.
- **Scope**: any word can be suspended, including one that is Not started — opening
  it creates its bare card as usual, and the Suspend action on that card suspends
  the word.
- **Export**: suspended cards are never in an exported package, even if a search
  with `status:suspended` is the last search made (the export button exports
  *approved* cards; a suspended card is never approved, and the export route also
  filters `suspended` as defense in depth).
- **Search**: a suspended word is dropped from every query result — text search,
  `status:progress,ready,approved`, `missing:`, `tag:`, the default list, the
  In progress / Ready tabs — unless the query contains a positive `status:suspended`
  term. `status:suspended` can combine with other terms (`status:suspended tag:x`).
- **Navigation**: `/api/step` (`neighbour`) re-runs the queue's search. With the
  exclusion above in the query layer, an empty-`q` walk (WALK_ALL) and any normal
  search skip suspended cards automatically; a queue from a `status:suspended`
  search walks them. The Continue flow (`/next`) never targets a suspended word.

### Open questions (confirm before or during Task 1)

1. **Reset word on a suspended card**: this plan assumes Reset deletes the card and
   the word returns to Not started **unsuspended** (suspension is a property of the
   card). The alternative — suspension sticking to the word across a reset — needs
   a separate word-keyed table and more migration risk.
2. **Confirm** that suspending from the card page of a not-started word (bare card
   created on open) is the intended way to suspend such a word.

## Required skills

Every task below **must** be done with the `tdd` and `laws-of-simplicity` skills
loaded and followed:

- **`tdd`**: red → green loop, one vertical slice per cycle (one test → one minimal
  implementation → repeat), tests only at the pre-agreed seams below, never
  tautological or implementation-coupled. No bulk test-writing up front.
- **`laws-of-simplicity`**: apply the 10 Laws at each stage (REDUCE the feature to
  the one flag and one search value; ORGANIZE around the existing status flow;
  LEARN via the README; CONTEXT — check export, navigation, tabs, Continue,
  backups; EMOTION/TRUST — the word must visibly and reliably vanish and come
  back). Score `simplicity_delta` after each task; DISCARD any approach that adds
  complexity for marginal gain. Trace the actual call path before declaring
  anything missing.

## Test seams (agreed up front, per the tdd skill)

| Seam | Catches | Misses |
|---|---|---|
| `Store` public API (`store.test.ts`) | suspension persists, approval taken back, unsuspend restores | SQL details (good) |
| `view` / `neighbour` / `continueTarget` (`words.test.ts`) | exclusion from every search, `status:suspended` visibility, navigation rules | internal row building |
| query `parse`/`run` (`parse.test.ts`, `run.test.ts`) | `status:suspended` parses; combination with other terms | wording of hints |
| `/api/export` (`route.test.ts`) | suspended never exported | bytes of apkg (covered by anki tests) |
| `/api/step` (`route.test.ts`) | next/previous skips or includes per `q` | browser behaviour |
| UI via cypress | button presence, aria, visible vanish/return | screen-reader nuance (reviewed manually) |

## Tasks

Each task: **red → green cycles with the `tdd` skill**, then a **review sub-agent**
runs (if sub-agents are unavailable, the main agent reviews adversarially itself),
reviewing **all changes that task made**, with special attention to **coherent
UI/UX and ARIA practices** (labels, roles, focus handling, keyboard paths, no
layout shift). The sub-agent hands its findings back; the main agent fixes **all
findings, even minor ones**, before moving to the next task.

### Task 1 — Data model and store

- Add a `suspended` boolean column to `notes`; bump `SCHEMA_VERSION` with a
  migration for existing databases (follow the existing migration pattern in
  `src/lib/store.ts`).
- `Store.suspend(id)` / `Store.unsuspend(id)` (or one `setSuspended`): sets the
  flag and takes approval back (`status` → `draft`) in the same transaction;
  unsuspend leaves `status` as `draft` (the card returns to progress/ready by its
  gaps). `allNotes`/`getNote` expose the flag on `Note`.
- Cycles: suspend persists + approval back → unsuspend → suspend on a draft →
  round-trip through an existing (older-schema) database.
- Review checkpoint.

### Task 2 — Status surface and search semantics

- `WordStatus` gains `"suspended"` (`src/lib/types.ts`); `wordStatus`
  (`src/lib/missing.ts`) returns it when the note is suspended.
- `fields.ts`: `status` accepts `suspended` (add to `STATUSES`, with its hint list).
- Exclusion: in `evaluate` (`src/lib/words.ts`), drop suspended rows from the
  candidate rows unless the parsed query carries a positive `status:suspended`
  test (parse once, e.g. a flag on `Query`). This one place gives the default list,
  tabs, text search, Continue and `neighbour` the same rule.
- Cycles: default list hides suspended → `status:suspended` shows them →
  `status:progress,ready,approved` hides them → combination `status:suspended
  tag:x` → text search hides them → Continue skips them.
- Review checkpoint.

### Task 3 — Export and navigation wiring

- `/api/export` filters out suspended notes (defense in depth next to the
  approved-only filter).
- `neighbour` / WALK_ALL: verify (with tests) that empty-`q` walks skip suspended
  cards and a `status:suspended` queue walks them; `continueFrom` never returns a
  suspended row.
- Cycles: export excludes an approved-then-suspended card → step next skips →
  step within `status:suspended` queue includes.
- Review checkpoint.

### Task 4 — API endpoint

- A route to suspend/unsuspend a card, e.g. `POST /api/notes/[id]/suspend` and
  `DELETE` on the same path (follow the style of the existing notes sub-routes,
  e.g. tags), returning the updated note like they do.
- Cycles: suspend → unsuspend → unknown id.
- Review checkpoint.

### Task 5 — Card editor UI

- A Suspend action in the card editor, placed with the other destructive-ish card
  actions (near Reset word), and an Unsuspend action shown when the card is
  suspended. Suspend asks for confirmation only when the card is approved (it
  takes the approval back); otherwise it acts at once, per the app's minimal-
  chrome style.
- While suspended, the card page shows the suspended state plainly (e.g. the
  bottom bar says so) and the word counter/status reflect it; the page stays
  fully editable so it can be fixed and unsuspended.
- ARIA: the action is a real `<button>` with an accessible name (visible label or
  `aria-label` matching the app's icon-button pattern), `aria-pressed`/state is
  not needed since the action swaps to Unsuspend; focus stays on the button after
  the state change; no layout shift (reuse the 24px icon-button pattern); the
  confirmation dialog follows the existing Reset dialog's role/focus behaviour.
- Cycles (cypress): suspend from the editor → the word vanishes from the default
  list → findable only via `status:suspended` → unsuspend → back everywhere;
  next/previous from a `status:suspended` queue stays inside the queue.
- Review checkpoint.

### Task 6 — Docs and final pass

- README: add `status:suspended` to the search-bar table (status row), and one
  sentence in the flow (step 2 or 4) describing Suspend word, the approval
  take-back, and that only `status:suspended` finds suspended words.
- Full regression: `pnpm test` (vitest), cypress run, `pnpm build`.
- Final review sub-agent over the whole diff (UI/UX + ARIA + the two skills'
  checklists), fix all findings.
