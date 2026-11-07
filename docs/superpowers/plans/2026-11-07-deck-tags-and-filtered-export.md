# Tags and a Search-Filtered Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans, inline, by one agent, with a fresh review agent after every task as set out in "How this plan is carried out". Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cards can carry tags such as `lesson_1`; the search bar finds cards by tag; an Export icon next to Search writes only the approved cards of the current search, so a subset of the deck (lessons 1-4) can be handed to classmates. The card page shows only the essentials: tags and the lexical details move into one dialog.

**Architecture:** Tags live in a new table `note_tags` and travel on `Note.tags`; a separate endpoint sets them so that tagging never touches approval. A new `tag:` field joins the search language. `GET /api/export?q=` runs the same search the home page runs and builds the deck from the approved cards it finds. The Export button leaves the menu and becomes an icon button in the search row. The card page's Details section and the new tag editor share one dialog, opened from an icon button in the card's top row.

**Tech Stack:** Next 15 route handlers and React 19 client components, better-sqlite3, Tailwind, Vitest, Cypress.

**Spec:** none separate; the decisions are below, as in the earlier plans. The owner approved previews of every screen change (the Export icon, and the dialog as "option A").

## How this plan is carried out

1. **Skills, before every task:** load `anthropic-skills:tdd` and `anthropic-skills:laws-of-simplicity` with the Skill tool, and follow them.
   - TDD: tests go only at the five seams listed under Decisions, which the owner confirmed. One failing test at a time, shown red before its code and green after it, expected values as literals from an independent source, never recomputed the way the code does. Refactoring is not part of the loop; it belongs to the review.
   - Laws of simplicity: run the checklist before starting a task and again before handing it to review; start from REDUCE (what here is not essential?). The task report names the `simplicity_delta` (-5 to +5) and why.
2. **One agent, one task after another.** The same agent (Claude) implements Tasks 1 to 6 in order. Nothing is implemented in parallel or by a subagent.
3. **A fresh review agent after each task.** When a task's implementation is committed, spawn a new review agent with the Agent tool that has none of the implementer's context. Its prompt holds the commit range of the task, the task's text, the Global Constraints and the Review Focus; it runs the `code-review` skill at level `high` on that range and reports its findings.
4. **Then the implementer fixes all of the issues the review found.** None is left for later. A finding that is a bug is fixed test first, at a confirmed seam. A finding that does not hold up against the code is answered in the task report with the evidence, not skipped silently. The fixes are committed apart from the task ("Fix the findings of the review of Task N").
5. **A task is done** when, after the fixes, `pnpm typecheck`, `pnpm test` and the Cypress specs named in the task are green and the fixes are committed and pushed. Only then does the next task start. There is one review per task; the whole branch is checked once more in "Finish".

**Before Task 1:** branch `claude/mufradat-deck-filtering-export-9pcyzi` at the latest commit, clean tree; `pnpm install`; `pnpm typecheck`, `pnpm test` and `pnpm e2e` green, and their counts written down. Run single Cypress specs with `node scripts/e2e.mjs run --spec <file>`; it rebuilds first.

## Decisions

Owner's (from the request and the answers):
1. Tags are added on the card page only; no bulk tagging, no pasted word lists, no list of tags already in use.
2. The search bar finds cards by tag. Export writes the approved cards of the current search; with nothing typed it writes every approved card, as today.
3. Export is an icon-only button, the download icon (a tray with a down arrow), in the search row to the right of Search. It leaves the ⋯ menu. Its hover text, set with the `title` attribute, is `Export <n> approved cards`; there is no text label and no explanation text.
4. Tag names have no spaces, like Anki: typing `Lesson 1` stores `lesson_1`.
5. `tag:` accepts `*` as a wildcard (`tag:lesson_*`), which makes up for the limit of 12 values per field; the limit itself stays.
6. The card page keeps only the essentials. Tags and Details (part of speech, gender, root, verb form) both move into one dialog, "Tags and details", opened from an icon button in the card's top row, right of Reset word.
7. Tags are not shown in the word list.
8. Tests go at five seams: (1) the store through `openStore(":memory:")`; (2) `view(store, q)`; (3) `buildApkg`; (4) route handlers in Vitest; (5) the browser in Cypress.
9. The process above.

Mine (overrule any of them):
- The "Tags and details" icon is a tag. The dialog is mostly about tags, and a tag reads at once; the details are found through the hover text and the dialog's own title. A gear says "app settings", sliders or an "i" say little, so I did not use them. Swapping it is a one-line change.
- Accessibility with the least ARIA: the two new icon buttons carry only `title` (it gives the accessible name and the hover text; an `aria-label` with the same text made Chromium also expose it as a description, so a screen reader may say it twice), and their icons are `aria-hidden`. The chip's remove buttons have no other name, so they get `aria-label="Remove tag <tag>"`. The existing Reset word button, which has both, is left alone.
- `*` matches any run of characters, including none; every other character, `_` and `.` included, is literal, so `lesson_*` means what it looks like. It is a wildcard, not a regular expression.
- Tags are lowercased on entry; matching ignores case anyway. A tag may not contain `,`, `"` or `*` (they are the search syntax), is at most 40 characters, and a card has at most 20.
- Changing tags does not take the approval back and does not change the card's edited time: a tag labels a card, it is not card content. So tagging does not move a card in `sort:-edited` or `edited:`.
- The tags are written into the Anki deck as Anki tags, after the existing `pos::` and `rank::` tags, so classmates can study one lesson in Anki.
- Tags are stored in `note_tags(note_id, tag)` with `ON DELETE CASCADE`, so Reset word removes them. `SCHEMA_VERSION` becomes 2: an older Mufradat refuses the newer workspace and backup, and this one opens older ones (the table is created on open).
- The deck keeps the configured name and ids, and the file name stays `arabic-deck-YYYY-MM-DD.apkg`. A classmate who imports lessons 1-4 and later 5-8 adds to one deck.
- The count and the deck both come from the last applied search; text typed in the bar and not yet searched changes neither. The icon is disabled at 0 and its hover text then reads `Export 0 approved cards`.
- The dialog focuses `Add tag` when it opens (otherwise the browser focuses the first tag's remove button) and a refused tag shows inside the dialog, because the dialog covers the bottom bar where other refusals appear.
- `-tag:*` lists the cards that have no tag (add `status:approved` to find approved ones still to tag).
- The review agent runs at `high`; each new test is mutation-proved (break the code, see it fail, restore), as in the earlier plans.

## Global Constraints

- No comments in code. Never the `any` type.
- `pnpm typecheck` and `pnpm test` green before every commit; the specs named in the task green; read exit statuses. README updated in the commit that changes behaviour.
- Copy, exact: icon button titles `Tags and details` and `Export <n> approved cards` (`Export 1 approved card`); dialog title `Tags and details of <lemma>`; headings `Tags` and `Details`; input label `Add tag`; hint `Press Enter to add a tag. A space becomes _.`; remove button label `Remove tag <tag>`; button `Done`; status `Preparing the deck…`; refusals `No approved card matches this search.`, `There are no approved cards to export.` (kept), `A tag cannot contain commas, quotes or *.`, `A tag is at most 40 characters.`, `A card has at most 20 tags.`, `tags must be a list of text.`
- Neither new icon button has an `aria-label` or a `role`; their SVGs have `aria-hidden="true"` and `focusable="false"`.
- The tags endpoint never changes a card's `status` or `updatedAt`. The export route writes nothing under the workspace, and the same cards (tags included) give the same bytes.
- The Cypress workspace is shared between specs: a test that tags cards removes its tags before it ends.

## Review Focus

Failure modes the main slices do not reach; each has its test in the task named.
1. Tagging an approved card leaves it approved and leaves its edited time alone (Task 1 store; Task 2 browser).
2. Export holds every approved match, not only the 30 on the page, and never a draft or a not-started word that the search also lists; the count on the icon is the number of cards the deck holds, both from `approvedIds`; text typed and not searched changes neither (Task 5 for the deck, Task 6 for the icon).
3. A search that cannot be read, or holds no approved card, exports nothing and says why; a blank search exports every approved card (Task 5; Task 6 for the disabled icon).
4. A database or backup made before tags opens and restores with no tags and nothing lost (Task 1).
5. Moving Details loses nothing: every detail field still saves from the dialog, and a refused tag is shown inside the dialog where it can be seen (Task 2).

## File Structure

Create: `src/lib/tags.ts`; `src/app/api/notes/[id]/tags/route.ts` and `route.test.ts`; `src/components/Tags.tsx`; `src/components/TagsAndDetailsDialog.tsx`; `src/components/ExportButton.tsx`; `src/lib/testing/deck.ts`; `cypress/e2e/tags.cy.ts`, `cypress/e2e/export.cy.ts`.

Modify: `src/lib/types.ts`, `src/lib/store.ts`, `src/lib/store.test.ts`, `src/lib/missing.test.ts`, `src/lib/words.ts`, `src/lib/words.test.ts`, `src/lib/query/fields.ts`, `src/lib/query/parse.test.ts`, `src/lib/anki.ts`, `src/lib/anki.test.ts`, `src/app/api/export/route.ts` and `route.test.ts`, `src/app/api/backups/[id]/restore/route.test.ts`, `src/components/CardScreen.tsx`, `src/components/Details.tsx`, `src/components/WordBar.tsx`, `src/components/Menu.tsx`, `src/app/layout.tsx`, `src/app/page.tsx`, `tailwind.config.js`, `cypress/e2e/card.cy.ts`, `scale.cy.ts`, `list.cy.ts`, `menu.cy.ts`, `README.md`.

---

### Task 1: Tags in the store and over HTTP

**Files:** Create `src/lib/tags.ts`, `src/app/api/notes/[id]/tags/route.ts`, `src/app/api/notes/[id]/tags/route.test.ts`. Modify `src/lib/types.ts`, `src/lib/store.ts`, `src/lib/store.test.ts`, `src/lib/missing.test.ts` (its hand-built notes gain `tags: []`), `src/app/api/backups/[id]/restore/route.test.ts`, `README.md`.

**Interfaces:**
- Produces: `Note.tags: string[]`, lowercase and ascending; `SCHEMA_VERSION = 2`.
- Produces: `store.setTags(id: string, tags: readonly string[]): Note` replaces the card's tags without touching `status` or `updatedAt`, and throws `There is no such card.` for an unknown id.
- Produces: `parseTags(input: unknown): { tags: string[] } | { error: string }` in `src/lib/tags.ts`. The input must be a list of strings; each entry is trimmed, lowercased, its whitespace runs become `_`, empty entries are dropped, duplicates collapse, the result is ascending. The errors are the exact texts under Global Constraints (`,` `"` `*`; more than 40 characters; more than 20 tags; not a list).
- Produces: `PUT /api/notes/<id>/tags` with `{ "tags": string[] }`: 200 `{ note }`, 400 `{ error }`; it mirrors `src/app/api/notes/[id]/route.ts` (`handle`, `readJson`, `fail`).

- [ ] **Step 1 (store, kept with the card):** test "keeps a card's tags, ascending, with the card": `s.setTags(id, ["b", "a"]).tags` and `s.getNote(id)!.tags` and `s.allNotes()[0].tags` are `["a", "b"]`; a bare card's tags are `[]`. Run `pnpm vitest run src/lib/store.test.ts`, expect red. Implement: `note_tags` in `SCHEMA`, `SCHEMA_VERSION = 2`, `Note.tags` filled in `build()` (grouped in `allNotes` the way forms are), `setTags` as a transaction after `need(id)`, no `touch`. Green.
- [ ] **Step 2 (store, approval):** test "tagging leaves an approved card approved and its edited time alone": approve a valid card as the neighbouring approval tests do, then `setTags`; `status` is `"approved"` and `updatedAt` equals its earlier value. Red or already green; green.
- [ ] **Step 3 (store, reset):** test "resets a tagged card": after `setTags(id, ["a"])`, `deleteNote(id)` does not throw and `getNote(id)` is `null`. It passes at once; prove it by mutation (drop `ON DELETE CASCADE`: the delete then fails on the foreign key).
- [ ] **Step 4 (store, older database):** test "opens a database made before tags": make a store in a directory, add a card, close it, open the file with better-sqlite3, `DROP TABLE note_tags`, `PRAGMA user_version = 1`, close; `openStore(dir)` again: the card is there with `tags` `[]` and `setTags` works. It passes at once (the table is created on open); prove it by mutation (`CREATE TABLE IF NOT EXISTS` becomes `CREATE TABLE`).
- [ ] **Step 5 (route, normalised):** in `tags/route.test.ts`, with the workspace stubbed as in `notes/[id]/route.test.ts`: test "saves the tags as stored": `PUT` `{ tags: ["Lesson 1", "lesson_2", "  Grammar  ", "grammar"] }` gives 200 and `note.tags` `["grammar", "lesson_1", "lesson_2"]`, and the store agrees. Red; implement `parseTags` and the route. Green.
- [ ] **Step 6 (route, refusals, one rule per cycle in this order):** `["a,b"]`, `["a\"b"]` and `["a*"]` give 400 with `A tag cannot contain commas, quotes or *.`; `["x".repeat(41)]` gives `A tag is at most 40 characters.`; 21 distinct tags give `A card has at most 20 tags.`; `{ tags: "a" }` and `{}` give `tags must be a list of text.`; after each refusal the card keeps the tags it had; `{ tags: [] }` clears them.
- [ ] **Step 7 (restore of an older backup):** in `restore/route.test.ts`: test "restores a backup made before tags existed": tag a card, take a backup with `createSnapshot`, rewrite that backup's `vocabulary.db` the way Step 4 does and its `manifest.json` to `schema: 1`, restore it through the route as the neighbouring tests do; the answer is 200 and the card is back with `tags` `[]`. Passes at once; mutation-prove (a restore that refuses `schema` below the current one).
- [ ] **Step 8:** README (flow item 2: Reset word also removes tags). `pnpm typecheck`, `pnpm test`, mutation-prove the new tests, commit "Keep tags on cards". Then the review and its fixes.

### Task 2: Tags and details in one dialog, off the card page

**Files:** Create `src/components/Tags.tsx`, `src/components/TagsAndDetailsDialog.tsx`, `cypress/e2e/tags.cy.ts`. Modify `src/components/CardScreen.tsx`, `src/components/Details.tsx`, `cypress/e2e/card.cy.ts`, `cypress/e2e/scale.cy.ts`, `README.md`.

**Interfaces:**
- Consumes: `Note.tags`, `PUT /api/notes/<id>/tags`.
- Produces: in the top row of `CardScreen.tsx`, right of the Reset word button, `button[title="Tags and details"]`: a 16px tag icon, classes `action-quiet min-w-control-sm px-2`, `aria-haspopup="dialog"`, no `aria-label`. It opens `<TagsAndDetailsDialog note open onClose save saveTags />`, a `<dialog>` built like `FormsDialog` (`showModal`, closes on Esc and on a click of the backdrop, `aria-labelledby`, `max-w-lg`, focus back on the trigger when it closes), titled `Tags and details of <lemma>`.
- Produces: the dialog holds, in order: a `Tags` heading; the chips (an `li` per tag with its text and a `button[aria-label="Remove tag <tag>"]`); `input[aria-label="Add tag"]`, which adds what is typed on Enter and when it loses focus (not while composing, like `InlineText`) and has the focus when the dialog opens; the hint; a `role="alert"` line for a refusal; a `Details` heading with the four fields; `Done`.
- Produces: `Details.tsx` renders only the four fields (no `<details>`, no `summary`); the card page has no Details section.
- Produces: `CardScreen.saveTags(tags: string[])`, queued with `later` like `save`: it shows `Saving…`, `Saved` or `Not saved`, replaces `note` with the answer, and sends a refusal's text to the dialog's `role="alert"` line instead of the bottom bar.

- [ ] **Step 1 (browser, details move):** first make the existing tests say the new thing, so they go red: in `card.cy.ts` "shows the card the way it will be learned" opens the dialog (click `button[title="Tags and details"]`) before reading Part of speech; "keeps the lexical fields in Details, closed until you open them" becomes "keeps the lexical fields in the dialog, out of the card page" (no `details` element and no `label` Part of speech on the page; the dialog shows Part of speech, Gender, Root and Verb form; selecting `adverb` shows `Saved`); "puts Reset word right of the word counter, with Details closed" drops its `details` line; "no longer has Reset word inside Details" becomes "has no Reset word in the dialog"; in `scale.cy.ts` "fits a card with four forms…" measures `cy.contains("button", "New example")` instead of `details` against the bar. Add, in `tags.cy.ts`, "has one icon button for tags and details, right of Reset word": it is visible, in the Reset word's row, has no `aria-label`, and a click opens a dialog titled `Tags and details of <lemma>`. Run `card.cy.ts`, `scale.cy.ts` and `tags.cy.ts`, expect red; implement the button, the dialog with the Details fields (the Tags part is empty for now) and the removal of the section. Green.
- [ ] **Step 2 (focus and Esc):** test "focuses Add tag when the dialog opens, and gives the focus back to the button on Esc". Implement the focus on open.
- [ ] **Step 3 (add):** test "adds a tag with Enter, shows it as stored and keeps it after a reload": type `Lesson 1` and press Enter in `Add tag`; the dialog holds a chip `lesson_1`; after `cy.reload()` and reopening it still does. The test removes the tag at its end. Implement `Tags`, `saveTags` and the chips.
- [ ] **Step 4 (remove):** test "removes a tag with its button": `Remove tag lesson_1` empties the chips, and a reload agrees.
- [ ] **Step 5 (approval):** test "leaves an approved card approved": approve a card through the API as `list.cy.ts` does, add a tag, and `cy.request("/api/backups")` still reports the same `body.check.approved`.
- [ ] **Step 6 (refusal):** test "says why a tag is refused, inside the dialog, and keeps the tags it had": type `a,b` and press Enter; the dialog's `[role="alert"]` reads `A tag cannot contain commas, quotes or *.` and the chips are unchanged.
- [ ] **Step 7 (accessibility):** test "has no accessibility problems in light and dark with the dialog open" with `cy.axe("dialog[open]")`, as the neighbouring specs do.
- [ ] **Step 8:** README (flow item 2: tags and the lexical fields are in the Tags and details dialog, opened from the tag icon beside Reset word). Run `tags.cy.ts`, `card.cy.ts`, `scale.cy.ts`, `keys.cy.ts`, `pnpm typecheck`, `pnpm test`; mutation-prove (the PUT also un-approves; the input ignores Enter; the refusal goes to the bottom bar) and commit. Then the review and its fixes.

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

### Task 6: The Export icon in the search row

**Files:** Create `src/components/ExportButton.tsx`, `cypress/e2e/export.cy.ts`. Modify `src/components/WordBar.tsx`, `src/components/Menu.tsx`, `src/app/layout.tsx`, `src/app/page.tsx`, `tailwind.config.js`, `cypress/e2e/menu.cy.ts`, `README.md`.

**Interfaces:**
- Consumes: `approvedIds`, `GET /api/export?q=`.
- Produces: `tailwind.config.js` `minWidth` gains `control: "var(--control-h)"`, so `min-w-control` is as wide as `min-h-control` is tall.
- Produces: `<ExportButton q={string} count={number} />`: `button[type="button"]` with classes `action min-h-control min-w-control px-0`, `title="Export <count> approved cards"` (`Export 1 approved card`), `disabled` at 0, no `aria-label`, no `role`, holding the download icon (an 18px SVG: the tray `M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4`, the arrow `M7 10l5 5 5-5` and `M12 15V3`, stroke `currentColor`, width 2, round caps, `aria-hidden="true"`, `focusable="false"`). As a sibling it renders a status line, `w-full px-1 text-sm text-quiet empty:hidden` with `role="status"`. A click sets `Preparing the deck…`, fetches `/api/export` with `q` as its parameter (none when `q` is blank), hands the Blob to the browser through a temporary link named from `Content-Disposition` (the code moves out of `Menu.tsx`), then clears the line; a refusal shows the server's text (via `refusal`).
- Produces: `WordBar` takes `children` and lays out a row `div.mt-4.flex.flex-wrap.items-center.gap-2` with the search `form` (`flex-1`, no longer carrying `mt-4`) and then `children`; the error line stays below. `page.tsx` passes `<ExportButton q={q} count={approvedIds(rows).length} />` as the child. `Menu` takes no props and has Backups, Prune audio and Keyboard shortcuts; `layout.tsx` renders `<Menu />` and no longer reads the counts.

- [ ] **Step 1 (browser, the main path):** in `export.cy.ts`: test "puts an export icon right of Search, named by how many approved cards the search holds, and exports exactly those": approve two cards and tag them `lesson_1`; on `/?q=tag:lesson_1` the search input, `Search` and the icon are in one row (the icon's centre within 2px of Search's and its left edge at or beyond Search's right edge), the icon's `title` is `Export 2 approved cards`, it has no `aria-label` and its SVG is `aria-hidden`; intercept `GET /api/export?q=tag%3Alesson_1`, click; one request, one new file in `downloads`, `workspaceExports` is `[]`. Red; implement `ExportButton`, `WordBar`, `page.tsx` and the `tailwind.config.js` line. Green.
- [ ] **Step 2 (the home page without a search):** test "counts every approved card for an empty search": the icon's `title` on `/` reads `Export <n> approved cards` with `n` equal to `body.check.approved` of `cy.request("/api/backups")`; the request carries no `q`.
- [ ] **Step 3 (ported from `menu.cy.ts`, each red before the code, then green):** "is disabled, with the title Export 0 approved cards, when the search holds none" (`/?q=tag:none_such`); "says Preparing the deck… under the row while it is built and nothing once it is on its way"; "says why when the export is refused and saves nothing"; "says Request failed (500). for an answer that is not JSON"; "says Request failed (400). when the refusal carries no message"; "keeps the icon in the row and the message under it, and does not scroll sideways, at 390px".
- [ ] **Step 4 (typed, not searched):** test "ignores text typed in the bar until it is searched": type `tag:x` without submitting; the icon's `title` is unchanged and a click requests the earlier `q`.
- [ ] **Step 5 (the menu):** in `menu.cy.ts` delete the export block, and the list test expects `["Backups", "Prune audio", "Keyboard shortcuts"]`; red until `Menu.tsx` loses Export and its `approved` prop.
- [ ] **Step 6 (accessibility):** test "has no accessibility problems on the home page with the icon, light and dark" with `cy.axe()`.
- [ ] **Step 7:** README (flow item 5 and "The search bar": Export is the icon right of Search). Run `export.cy.ts`, `menu.cy.ts`, `backups.cy.ts`, `housekeeping.cy.ts`, `keys.cy.ts`, `scale.cy.ts`, `list.cy.ts`, `pnpm typecheck`, `pnpm test`; mutation-prove (Export ignores `q`; the count ignores the search; an `aria-label` is added) and commit. Then the review and its fixes.

---

## Finish

Whole-suite run in the normal order (`pnpm typecheck`, `pnpm test`, `pnpm e2e`), compared with the counts from before Task 1. A read of the whole branch diff against the owner's nine decisions and the rules (no comments, no `any`). Push `claude/mufradat-deck-filtering-export-9pcyzi`; no pull request unless the owner asks. Then the branch menu of `superpowers:finishing-a-development-branch`.
