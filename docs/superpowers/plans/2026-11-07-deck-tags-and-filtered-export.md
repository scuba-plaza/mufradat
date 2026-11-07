# Tags and a Search-Filtered Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans, inline, by one agent, with a fresh review agent after every task as set out in "How this plan is carried out". Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cards can carry tags such as `lesson_1`; the search bar finds cards by tag; Export on the home page writes only the approved cards of the current search, so a subset of the deck (lessons 1-4) can be handed to classmates.

**Architecture:** Tags live in a new table `note_tags` and travel on `Note.tags`; a separate endpoint sets them so that tagging never touches approval. A new `tag:` field joins the search language. `GET /api/export?q=` runs the same search the home page runs and builds the deck from the approved cards it finds. The Export button moves from the menu to the home page, under the search bar.

**Tech Stack:** Next 15 route handlers and React 19 client components, better-sqlite3, Vitest, Cypress.

**Spec:** none separate; the decisions are below, as in the earlier plans.

## How this plan is carried out

1. **Skills, before every task:** load `anthropic-skills:tdd` and `anthropic-skills:laws-of-simplicity` with the Skill tool, and follow them.
   - TDD: tests go only at the five seams listed under Decisions, which the owner confirmed. One failing test at a time, shown red before its code and green after it, expected values as literals from an independent source, never recomputed the way the code does. Refactoring is not part of the loop; it belongs to the review.
   - Laws of simplicity: run the checklist before starting a task and again before handing it to review; start from REDUCE (what here is not essential?). The task report names the `simplicity_delta` (-5 to +5) and why.
2. **One agent, one task after another.** The same agent (Claude) implements Tasks 1 to 6 in order. Nothing is implemented in parallel or by a subagent.
3. **A fresh review agent after each task.** When a task's implementation is committed, spawn a new review agent with the Agent tool that has none of the implementer's context. Its prompt holds the commit range of the task, the task's text, the Global Constraints and the Review Focus; it runs the `code-review` skill at level `high` on that range and reports its findings.
4. **Then the implementer fixes all of the issues the review found.** None is left for later. A finding that is a bug is fixed test first, at a confirmed seam. A finding that does not hold up against the code is answered in the task report with the evidence, not skipped silently. The fixes are committed apart from the task ("Fix the findings of the review of Task N").
5. **A task is done** when, after the fixes, `pnpm typecheck`, `pnpm test` and the Cypress specs named in the task are green and the fixes are committed and pushed. Only then does the next task start. There is one review per task; the whole branch is checked once more in "Finish".

**Before Task 1:** branch `claude/mufradat-deck-filtering-export-9pcyzi` at `5c2ee28`, clean tree; `pnpm install`; `pnpm typecheck`, `pnpm test` and `pnpm e2e` green, and their counts written down. Run single Cypress specs with `node scripts/e2e.mjs run --spec <file>`; it rebuilds first.

## Decisions

Owner's (from the request and the answers):
1. Tags are added on the card page only; no bulk tagging and no pasted word lists.
2. The search bar finds cards by tag. Export writes the approved cards of the current search; with nothing typed it writes every approved card, as today.
3. The Export button sits under the search bar on the home page and shows how many approved cards the search holds; it leaves the menu.
4. Tag names have no spaces, like Anki: typing `Lesson 1` stores `lesson_1`.
5. `tag:` accepts `*` as a wildcard (`tag:lesson_*`), which makes up for the limit of 12 values per field; the limit itself stays.
6. Tests go at five seams: (1) the store through `openStore(":memory:")`; (2) `view(store, q)`; (3) `buildApkg`; (4) route handlers in Vitest; (5) the browser in Cypress.
7. The process above.

Mine (overrule any of them):
- `*` matches any run of characters, including none; every other character, `_` and `.` included, is literal, so `lesson_*` means what it looks like. It is a wildcard, not a regular expression.
- Tags are lowercased on entry; matching ignores case anyway. A tag may not contain `,`, `"` or `*` (they are the search syntax), is at most 40 characters, and a card has at most 20.
- Changing tags does not take the approval back and does not change the card's edited time: a tag labels a card, it is not card content. So tagging does not move a card in `sort:-edited` or `edited:`.
- The tags are written into the Anki deck as Anki tags, after the existing `pos::` and `rank::` tags, so classmates can study one lesson in Anki.
- Tags are stored in `note_tags(note_id, tag)` with `ON DELETE CASCADE`, so Reset word removes them. `SCHEMA_VERSION` becomes 2: an older Mufradat refuses the newer workspace and backup, and this one opens older ones (the table is created on open).
- The deck keeps the configured name and ids, and the file name stays `arabic-deck-YYYY-MM-DD.apkg`. A classmate who imports lessons 1-4 and later 5-8 adds to one deck.
- The button reads `Export 312 approved cards` (`Export 1 approved card`, `Export 0 approved cards`, disabled at 0). The count and the deck both come from the last applied search; text typed in the bar and not yet searched changes neither.
- `-tag:*` lists the cards that have no tag (add `status:approved` to find approved ones still to tag).
- The review agent runs at `high`; each new test is mutation-proved (break the code, see it fail, restore), as in the earlier plans.

## Global Constraints

- No comments in code. Never the `any` type.
- `pnpm typecheck` and `pnpm test` green before every commit; the specs named in the task green; read exit statuses. README updated in the commit that changes behaviour.
- Copy, exact: section `Tags`; input label `Add tag`; remove button label `Remove tag <tag>`; suggestion button label `Add tag <tag>`; export button `Export <n> approved cards`; status `Preparing the deck…`; refusals `No approved card matches this search.`, `There are no approved cards to export.` (kept), `A tag cannot contain commas, quotes or *.`, `A tag is at most 40 characters.`, `A card has at most 20 tags.`, `tags must be a list of text.`
- The tags endpoint never changes a card's `status` or `updatedAt`. The export route writes nothing under the workspace, and the same cards (tags included) give the same bytes.
- The Cypress workspace is shared between specs: a test that tags cards removes its tags before it ends.

## Review Focus

Failure modes the main slices do not reach; each has its test in the task named.
1. Tagging an approved card leaves it approved and leaves its edited time alone (Task 1 store; Task 2 browser).
2. Export holds every approved match, not only the 30 on the page, and never a draft or a not-started word that the search also lists (Task 5).
3. A search that cannot be read, or holds no approved card, exports nothing and says why; a blank search exports every approved card (Task 5; Task 6 for the disabled button).
4. Text typed in the bar and not searched does not change what Export exports, and the count on the button is the number of cards the deck holds, both coming from `approvedIds` (Task 5 for the deck, Task 6 for the button).
5. A database or backup made before tags opens and restores with no tags and nothing lost (Task 1).

## File Structure

Create: `src/lib/tags.ts`; `src/app/api/notes/[id]/tags/route.ts` and `route.test.ts`; `src/components/Tags.tsx`; `src/components/ExportButton.tsx`; `src/lib/testing/deck.ts`; `cypress/e2e/tags.cy.ts`, `cypress/e2e/export.cy.ts`.

Modify: `src/lib/types.ts`, `src/lib/store.ts`, `src/lib/store.test.ts`, `src/lib/missing.test.ts`, `src/lib/words.ts`, `src/lib/words.test.ts`, `src/lib/query/fields.ts`, `src/lib/query/parse.test.ts`, `src/lib/anki.ts`, `src/lib/anki.test.ts`, `src/app/api/export/route.ts` and `route.test.ts`, `src/app/api/backups/[id]/restore/route.test.ts`, `src/components/CardScreen.tsx`, `src/app/cards/[id]/page.tsx`, `src/components/Menu.tsx`, `src/app/layout.tsx`, `src/app/page.tsx`, `cypress/e2e/list.cy.ts`, `cypress/e2e/menu.cy.ts`, `README.md`.

---

### Task 1: Tags in the store and over HTTP

**Files:** Create `src/lib/tags.ts`, `src/app/api/notes/[id]/tags/route.ts`, `src/app/api/notes/[id]/tags/route.test.ts`. Modify `src/lib/types.ts`, `src/lib/store.ts`, `src/lib/store.test.ts`, `src/lib/missing.test.ts` (its hand-built notes gain `tags: []`), `src/app/api/backups/[id]/restore/route.test.ts`, `README.md`.

**Interfaces:**
- Produces: `Note.tags: string[]`, lowercase and ascending; `SCHEMA_VERSION = 2`.
- Produces: `store.setTags(id: string, tags: readonly string[]): Note` replaces the card's tags without touching `status` or `updatedAt`, and throws `There is no such card.` for an unknown id; `store.knownTags(): string[]` is every tag in use, once, ascending.
- Produces: `parseTags(input: unknown): { tags: string[] } | { error: string }` in `src/lib/tags.ts`. The input must be a list of strings; each entry is trimmed, lowercased, its whitespace runs become `_`, empty entries are dropped, duplicates collapse, the result is ascending. The errors are the exact texts under Global Constraints (`,` `"` `*`; more than 40 characters; more than 20 tags; not a list).
- Produces: `PUT /api/notes/<id>/tags` with `{ "tags": string[] }`: 200 `{ note }`, 400 `{ error }`; it mirrors `src/app/api/notes/[id]/route.ts` (`handle`, `readJson`, `fail`).

- [ ] **Step 1 (store, kept with the card):** test "keeps a card's tags, ascending, with the card": `s.setTags(id, ["b", "a"]).tags` and `s.getNote(id)!.tags` and `s.allNotes()[0].tags` are `["a", "b"]`; a bare card's tags are `[]`. Run `pnpm vitest run src/lib/store.test.ts`, expect red. Implement: `note_tags` in `SCHEMA`, `SCHEMA_VERSION = 2`, `Note.tags` filled in `build()` (grouped in `allNotes` the way forms are), `setTags` as a transaction after `need(id)`, no `touch`. Green.
- [ ] **Step 2 (store, approval):** test "tagging leaves an approved card approved and its edited time alone": approve a valid card as the neighbouring approval tests do, then `setTags`; `status` is `"approved"` and `updatedAt` equals its earlier value. Red or already green; green.
- [ ] **Step 3 (store, known tags and reset):** test "lists each tag in use once and forgets the tags of a deleted card": card one `["a", "b"]`, card two `["b"]`, `knownTags()` is `["a", "b"]`; after `deleteNote` of card one it is `["b"]`. Implement `knownTags`.
- [ ] **Step 4 (store, older database):** test "opens a database made before tags": make a store in a directory, add a card, close it, open the file with better-sqlite3, `DROP TABLE note_tags`, `PRAGMA user_version = 1`, close; `openStore(dir)` again: the card is there with `tags` `[]` and `setTags` works. It passes at once (the table is created on open); prove it by mutation (`CREATE TABLE IF NOT EXISTS` becomes `CREATE TABLE`).
- [ ] **Step 5 (route, normalised):** in `tags/route.test.ts`, with the workspace stubbed as in `notes/[id]/route.test.ts`: test "saves the tags as stored": `PUT` `{ tags: ["Lesson 1", "lesson_2", "  Grammar  ", "grammar"] }` gives 200 and `note.tags` `["grammar", "lesson_1", "lesson_2"]`, and the store agrees. Red; implement `parseTags` and the route. Green.
- [ ] **Step 6 (route, refusals, one rule per cycle in this order):** `["a,b"]`, `["a\"b"]` and `["a*"]` give 400 with `A tag cannot contain commas, quotes or *.`; `["x".repeat(41)]` gives `A tag is at most 40 characters.`; 21 distinct tags give `A card has at most 20 tags.`; `{ tags: "a" }` and `{}` give `tags must be a list of text.`; after each refusal the card keeps the tags it had; `{ tags: [] }` clears them.
- [ ] **Step 7 (restore of an older backup):** in `restore/route.test.ts`: test "restores a backup made before tags existed": tag a card, take a backup with `createSnapshot`, rewrite that backup's `vocabulary.db` the way Step 4 does and its `manifest.json` to `schema: 1`, restore it through the route as the neighbouring tests do; the answer is 200 and the card is back with `tags` `[]`. Passes at once; mutation-prove (a restore that refuses `schema` below the current one).
- [ ] **Step 8:** README (flow item 2: tags are set on the card page; Reset word also removes tags). `pnpm typecheck`, `pnpm test`, mutation-prove the new tests, commit "Keep tags on cards". Then the review and its fixes.

### Task 2: The tag editor on the card page

**Files:** Create `src/components/Tags.tsx`, `cypress/e2e/tags.cy.ts`. Modify `src/components/CardScreen.tsx`, `src/app/cards/[id]/page.tsx`, `README.md`.

**Interfaces:**
- Consumes: `Note.tags`, `store.knownTags()`, `PUT /api/notes/<id>/tags`.
- Produces: `CardScreen` takes `knownTags: string[]` (from `page.tsx`) and gains `saveTags(tags: string[])`, which goes through the same `later` queue as `save`, shows `Saving…`/`Saved`/`Not saved`, replaces `note` with the answer and shows a refusal where other refusals show.
- Produces: `<Tags tags={string[]} known={string[]} onChange={(tags: string[]) => void} />`, a `section[aria-label="Tags"]` between the example block and `Details`: one `li` per tag with a button labelled `Remove tag <tag>`; an `input` labelled `Add tag` that adds what is typed on Enter or when it loses focus (not while composing, like `InlineText`); one button labelled `Add tag <tag>` for each known tag the card does not have.

- [ ] **Step 1 (browser, add):** in `tags.cy.ts`: test "adds a tag with Enter, shows it as stored and keeps it after a reload": open a card, type `Lesson 1` and press Enter in `Add tag`; the section holds `lesson_1`; after `cy.reload()` it still does. Run `node scripts/e2e.mjs run --spec cypress/e2e/tags.cy.ts`, expect red; implement `Tags`, `saveTags` and the page prop; green. The test removes the tag at its end.
- [ ] **Step 2 (remove):** test "removes a tag with its button": `Remove tag lesson_1` empties the section, and a reload agrees.
- [ ] **Step 3 (suggestions):** test "offers the tags used on other cards and adds one with a click": tag another card `lesson_2` through `cy.request("PUT", ...)`, open this card, click `Add tag lesson_2`; the tag shows and the suggestion is gone.
- [ ] **Step 4 (approval):** test "leaves an approved card approved": approve a card through the API as `list.cy.ts` does, add a tag, and `cy.request("/api/backups")` still reports the same `body.check.approved`.
- [ ] **Step 5 (refusal):** test "says why a tag is refused and keeps the tags it had": type `a,b` and press Enter; the page shows `A tag cannot contain commas, quotes or *.` and the section is unchanged.
- [ ] **Step 6 (accessibility):** test "has no accessibility problems in light and dark" with `cy.axe()`, as the neighbouring specs do.
- [ ] **Step 7:** README (flow item 2: how to tag). Run `tags.cy.ts`, `card.cy.ts`, `keys.cy.ts`, `pnpm typecheck`, `pnpm test`; mutation-prove (the PUT also un-approves; the input ignores Enter) and commit. Then the review and its fixes.

### Task 3: `tag:` in the search bar

**Files:** Modify `src/lib/types.ts` (`WordRow.tags`), `src/lib/words.ts`, `src/lib/query/fields.ts`, `src/lib/words.test.ts` (its `blank` row gains `tags: []`), `src/lib/query/parse.test.ts`, `cypress/e2e/list.cy.ts`, `README.md`.

**Interfaces:**
- Consumes: `Note.tags`.
- Produces: `WordRow.tags: string[]` (`[]` for a word with no card), set in `fromCard` and `notStarted`; `FIELDS.tag`, listed after `edited`. Each value matches a whole tag, ignoring case; `*` is a wildcard and every other character is literal. A value that matches nothing is an empty list, not an error. The limit of 12 values stays.

- [ ] **Step 1:** in `words.test.ts` build four bare cards with `openCard` and `setTags`: A `["lesson_1"]`, B `["grammar", "lesson_2"]`, C `["lesson_10"]`, D none. Test "finds cards by tag": `view(s, "tag:lesson_1").rows` are A only, not C. Expect red (`Unknown field "tag"`); implement `WordRow.tags` and the field. Green. In the same cycle change the unknown-field expectation in `parse.test.ts` to `Unknown field "colour". Fields: status, pos, rank, root, gender, verbform, missing, has, example, edited, tag, sort.`
- [ ] **Step 2:** test "takes * as a wildcard": `tag:lesson_*` gives A, B, C in rank order.
- [ ] **Step 3:** test "ignores case and takes a comma as or": `tag:LESSON_1,grammar` gives A, B.
- [ ] **Step 4:** test "finds the cards without a tag and never lists a word without a card for tag:*": `-tag:* status:progress` gives D only; `tag:*` gives A, B, C and no other row.
- [ ] **Step 5:** test "treats every character but * literally": a card tagged `abc`; `tag:a.c` gives no rows, `tag:a*c` gives that card.
- [ ] **Step 6 (browser):** in `list.cy.ts`, test "filters the list by tag": tag two cards `lesson_1` through the API, search `tag:lesson_*` in the bar; the list is exactly those two; remove the tags afterwards.
- [ ] **Step 7:** README (the table row for `tag:` with `lesson_1`, `lesson_*`, `-tag:*`; one sentence on the wildcard). Run `list.cy.ts`, `pnpm typecheck`, `pnpm test`; mutation-prove (regex characters unescaped; the match not anchored) and commit. Then the review and its fixes.

### Task 4: The deck carries the tags

**Files:** Modify `src/lib/anki.ts`, `src/lib/anki.test.ts`, `README.md`.

**Interfaces:** in `buildApkg` a note's tags column is ` pos::<pos> rank::<bucket> <tag> <tag> ` (the tags in the order of `note.tags`), leaving out a tag the column already holds.

- [ ] **Step 1:** test "writes a card's tags after its pos and rank tags": from `approvedStore()` call `s.setTags(id, ["lesson_1", "grammar"])` and build; the note's `tags` are ` pos::noun rank::0001-0500 grammar lesson_1 ` (the card has rank 120). Red; implement. Green.
- [ ] **Step 2:** test "does not repeat a tag the deck already has": `setTags(id, ["pos::noun"])` leaves ` pos::noun rank::0001-0500 `.
- [ ] **Step 3:** test "gives the same bytes whatever order the tags were set in": build, `setTags` the same tags in the other order, build again; the buffers are equal.
- [ ] **Step 4:** README (Export: the tags go into the deck). `pnpm typecheck`, `pnpm test`, mutation-prove, commit. Then the review and its fixes.

### Task 5: Export follows the search (the route)

**Files:** Create `src/lib/testing/deck.ts`. Modify `src/lib/words.ts`, `src/lib/anki.test.ts` (imports `unzip` from the new file instead of defining it), `src/app/api/export/route.ts`, `src/app/api/export/route.test.ts`, `README.md`.

**Interfaces:**
- Produces: `approvedIds(rows: WordRow[]): string[]` in `src/lib/words.ts`: the ids of the rows whose status is `approved`; the route and the home page both use it, so the count and the deck cannot disagree.
- Produces: `GET /api/export?q=<search>` as `GET(req: Request)`; `q` absent or blank means every approved card. The deck holds `store.getNote(id)` for each id in `approvedIds(view(store, q).rows)`. 400 `{ error }`: the parser's own message for a search that cannot be read; `There are no approved cards to export.` when `q` is blank and nothing is approved; `No approved card matches this search.` otherwise; the missing-clip refusal as now.
- Produces: `unzip(buf: Buffer): Map<string, Buffer>` (moved from `anki.test.ts`) and `deckGuids(apkg: Buffer): string[]`, the note guids of a deck in ascending order, in `src/lib/testing/deck.ts`.

- [ ] **Step 1:** the existing export tests call `GET()`; change them to `GET(new Request("http://127.0.0.1/api/export"))` (they stay green). Write `deck.ts` with `unzip` moved and `deckGuids` added.
- [ ] **Step 2:** test "exports only the approved cards the search matches": approved X and Y tagged `lesson_1`, approved Z tagged `lesson_2`, a draft W tagged `lesson_1`; `?q=tag:lesson_1` gives 200 and `deckGuids` is X and Y ascending. Red (every approved card comes back); implement `approvedIds` and the route. Green.
- [ ] **Step 3:** test "exports every match, not only the first page": 31 approved cards tagged `lesson_1` and one draft; the deck holds 31 guids. Passes at once; mutation-prove (slice the ids to 30).
- [ ] **Step 4:** test "exports every approved card for a blank search": `?q=` and `?q=%20%20` both hold every approved card.
- [ ] **Step 5:** tests "refuses with the reason": `?q=pos:animal` gives 400 with `pos: expects noun, verb, adj, adv, prep, conj, particle, pron, num, intj, phrase or det, not "animal".`; `?q=tag:none_such` gives 400 `No approved card matches this search.`; a search that matches only a draft gives the same.
- [ ] **Step 6:** README (Export follows the search). `pnpm typecheck`, `pnpm test`, mutation-prove and commit. Then the review and its fixes.

### Task 6: The Export button under the search bar

**Files:** Create `src/components/ExportButton.tsx`, `cypress/e2e/export.cy.ts`. Modify `src/components/Menu.tsx`, `src/app/layout.tsx`, `src/app/page.tsx`, `cypress/e2e/menu.cy.ts`, `README.md`.

**Interfaces:**
- Consumes: `approvedIds`, `GET /api/export?q=`.
- Produces: `<ExportButton q={string} count={number} />`: a button reading `Export <count> approved card(s)`, disabled at 0, and a `role="status"` line under it. A click sets `Preparing the deck…`, fetches `/api/export` with `q` as its parameter, hands the Blob to the browser through a temporary link named from `Content-Disposition` (the code moves out of `Menu.tsx`), then clears the line; a refusal shows the server's text (via `refusal`).
- Produces: `page.tsx` renders it after `WordBar` with `q` and `approvedIds(rows).length`. `Menu` takes no props and has Backups, Prune audio and Keyboard shortcuts; `layout.tsx` renders `<Menu />` and no longer reads the counts.

- [ ] **Step 1 (browser, the main path):** in `export.cy.ts`: test "shows how many approved cards the search holds and exports exactly those": approve two cards and tag them `lesson_1`; `/?q=tag:lesson_1` shows `Export 2 approved cards`; intercept `GET /api/export?q=tag%3Alesson_1`, click; one request, one new file in `downloads`, `workspaceExports` is `[]`. Red; implement `ExportButton` and `page.tsx`. Green.
- [ ] **Step 2 (the home page without a search):** test "counts every approved card for an empty search": the number on `/` equals `body.check.approved` of `cy.request("/api/backups")`; the request carries no `q`.
- [ ] **Step 3 (ported from `menu.cy.ts`, each red before the code, then green):** "is disabled and reads Export 0 approved cards when the search holds none" (`/?q=tag:none_such`); "says Preparing the deck… while it is built and nothing once it is on its way"; "says why when the export is refused and saves nothing"; "says Request failed (500). for an answer that is not JSON"; "says Request failed (400). when the refusal carries no message"; "puts a message on its own row under the button and does not scroll sideways at 390px".
- [ ] **Step 4 (typed, not searched):** test "ignores text typed in the bar until it is searched": type `tag:x` without submitting; the button's text is unchanged and a click requests the earlier `q`.
- [ ] **Step 5 (the menu):** in `menu.cy.ts` delete the export block, and the list test expects `["Backups", "Prune audio", "Keyboard shortcuts"]`; red until `Menu.tsx` loses Export and its `approved` prop.
- [ ] **Step 6:** README (flow item 5 and "The search bar": Export is under the search bar). Run `export.cy.ts`, `menu.cy.ts`, `backups.cy.ts`, `housekeeping.cy.ts`, `keys.cy.ts`, `scale.cy.ts`, `pnpm typecheck`, `pnpm test`; mutation-prove (Export ignores `q`; the count ignores the search) and commit. Then the review and its fixes.

---

## Finish

Whole-suite run in the normal order (`pnpm typecheck`, `pnpm test`, `pnpm e2e`), compared with the counts from before Task 1. A read of the whole branch diff against the owner's seven decisions and the rules (no comments, no `any`). Push `claude/mufradat-deck-filtering-export-9pcyzi`; no pull request unless the owner asks. Then the branch menu of `superpowers:finishing-a-development-branch`.
