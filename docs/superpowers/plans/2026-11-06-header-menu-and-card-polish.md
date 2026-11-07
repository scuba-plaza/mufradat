# Header Menu and Card Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One menu in the header holds every workspace tool (export, backups, unused audio, keyboard shortcuts); Reset word becomes a small icon button beside the word counter; every destructive confirmation has a red confirm button; the home list shows 30 rows a page; a report-only accessibility and UX sweep lists what else should change.

**Architecture:** A client component `Menu.tsx` in the layout owns which dialog is open and renders the trigger, an anchored popover of items (the `.options` pattern `Clip.tsx` already uses) and the three existing dialogs, which become controlled (`open`, `onClose`) like `FormsDialog`. The layout passes the approved count, read from `store.counts()`. No new routes.

**Tech Stack:** Next 15.5.29 app router, React 19, Tailwind with CSS-variable colour tokens, Cypress (the only seam for this work), `axe-core` as a dev-only dependency.

**Spec:** none separate. The owner's decisions are in "Decisions" below, and rulings made while executing go in the ledger as `Ruling:` lines.

**Skills:** load `anthropic-skills:tdd` and `anthropic-skills:laws-of-simplicity` before each task, and tell every reviewer to use both. Tests go only at the Cypress seam (seam 6 of the earlier plan, confirmed by the owner), one failing test at a time, vertical slices, expected values as literals.

**Execution method (carried over from the owner's earlier instruction; confirm):** one task at a time, a fresh reviewer agent (opus, both skills) after each task, fixes before the next task, a ledger in `.superpowers/sdd/2026-11-06-header-menu-and-card-polish/`, commits on `claude/funny-hawking-mz7k1n`, no pull request unless asked.

**Before Task 1:** the branch is at `294fa48` with a clean tree; `pnpm typecheck` and `pnpm test` are green (304 tests). Run e2e specs with `node scripts/e2e.mjs run --spec <files>`; it rebuilds first.

## Decisions

Owner's:
1. The menu button shows a "⋯" icon (not "Menu" text, not "?").
2. Delete unused audio… moves into the menu, so the home page has no footer.
3. `axe-core` may be added as a dev dependency.
4. The sweep is report only: no fixes from it, a written list.
5. Reset word becomes a small icon button right of "Word X of N" at the top of a card.
6. Destructive confirmations get a red confirm button in a tone from the existing palette.
7. The list on the home page shows 30 rows a page (it is 100 now), so 167 pages for the 5000 words.

Mine (overrule any of them at plan review):
- **Export goes in the menu, not the header.** It is used now and then, not while editing a card; a header button would show on every card screen where a stray click starts a download; on a phone the header has room for "Export" but not the count or the reason it is disabled; in the menu it keeps the old full text and is one click from the top instead of the bottom of a 100-row list.
- **Menu pattern: popover disclosure**, not `role="menu"`. Tab moves through the items, Escape and a click outside close it and return the focus. `role="menu"` would promise arrow-key navigation and typeahead that the app does not have.
- The red is the existing `--danger` token (`193 63 58` light, `229 115 115` dark) as the background with `--on-primary` text (white, and `25 25 25` in dark): about 5.2:1 and 5.9:1, both above 4.5:1. No new colour token.
- **Red:** Reset, Delete unused audio's Delete, the Delete of a backup and Restore. Restore was first kept dark because it can be undone from the "Before a restore" backup; the Task 5 reviewer pointed out that Reset is undone the same way and is red, and that Restore replaces the whole workspace, so it is red too (a one-class change if the owner disagrees).
- The unlabeled `<nav>` around the old `?` button is dropped; a lone menu button is not navigation.
- The export message "Exported 1 cards" gets its plural fixed with `plural()`.
- The explanatory line under Reset word in Details ("Deletes this card, so that the word starts over.") is dropped; the dialog already says it.

**Open item for the owner, Nix:** the flake pins `pnpmDeps.hash`. Adding `axe-core` changes `pnpm-lock.yaml`, so `nix build` fails with a hash mismatch until the hash is refreshed. This container cannot do that: Nix's build sandbox rejects the proxy certificate (`SELF_SIGNED_CERT_IN_CHAIN`, seen in a baseline build of the current branch), so I cannot compute the hash or confirm the Nix build here. If axe-core stays, the final message gives the one command to run on the NixOS laptop and where to paste the `got:` hash. If you would rather not owe that, use axe-core from a scratch directory for the sweep only and keep the repo's checks hand-written; that changes only Task 1 step 1 and the committed `cy.axe` calls.

## Global Constraints

- No comments in code. Never the `any` type. Temporary files in tests are removed.
- `pnpm typecheck` and `pnpm test` green before every commit; the e2e specs named in the task green; read exit statuses, not only the summary line. README updated in the commit that changes behaviour.
- Reuse `.options` and `.options-item`; the only new CSS is `.options-end`, `.action-danger` and `flex-wrap` on `.topbar-inner`.
- Copy, exact:
  - Menu button: `aria-label="Menu"`, a three-dot icon with `aria-hidden`.
  - Items in this order: `Export 3 approved cards to Anki` (`Export 1 approved card to Anki` for one), or, disabled, `Export to Anki (nothing approved yet)`; `Backups…`; `Delete unused audio…`; `Keyboard shortcuts`.
  - Export message: `Exported 3 cards to <path>` / `Exported 1 card to <path>`.
  - Reset button: `aria-label="Reset word"` and `title="Reset word"`, a counter-clockwise arrow icon with `aria-hidden`.
- Every dialog opened from the menu gives the focus back to the menu button when it closes, by Escape, Close, Cancel or a click on the backdrop.
- Layout: the menu is the last item of `.topbar-inner`, its popover ends level with the button's right edge and stays inside the window at 390px and 1920px.

## Review Focus

Failure modes the tasks' main slices do not reach, most likely first. Each has its test in the task named.

1. Focus after a dialog opened from the menu closes: it must land on the menu button, not the page (Tasks 1 and 2).
2. The popover at 390px: not clipped on the right, no sideways page scroll (Task 1), also with a long export path showing (Task 3).
3. Export with nothing approved (disabled, says why), with the server refusing (the message shows, no download starts), and with exactly one card (singular) (Task 3).
4. Pages that render the layout without a card, such as the not-found page: the menu still opens (Task 3).
5. Reset on a card whose word is outside the frequency list, where the counter is empty: the button is still there and the dialog still says the word leaves the list (Task 4).

## File Structure

Create:
- `src/components/Menu.tsx`: the trigger, the popover, the export action and message, which dialog is open.
- `cypress/e2e/menu.cy.ts`: the menu's own behaviour.
- `docs/superpowers/reports/2026-11-06-ui-ux-accessibility-sweep.md`: Task 7's report.

Modify:
- `src/app/layout.tsx`: renders `<Menu approved={…} />` in the header.
- `src/components/Shortcuts.tsx`, `Backups.tsx`, `UnusedAudio.tsx`: controlled dialogs with no trigger of their own.
- `src/app/page.tsx`: the footer goes; `PAGE_SIZE` becomes 30.
- `src/components/CardScreen.tsx`, `Details.tsx`, `ResetDialog.tsx`: the reset button moves; the confirm button turns red.
- `src/app/globals.css`: `.options-end`, `.action-danger`, `.topbar-inner` wraps.
- `cypress/support/e2e.ts`: `cy.axe`, `cy.menuItem`, `cy.colorScheme`.
- `cypress/e2e/keys.cy.ts`, `backups.cy.ts`, `housekeeping.cy.ts`, `card.cy.ts`, `list.cy.ts`, `look.cy.ts`.
- `README.md`, `package.json`, `pnpm-lock.yaml`.

Delete: `src/components/ExportButton.tsx`.

---

### Task 1: The menu, with Keyboard shortcuts in it

**Files:** Create `src/components/Menu.tsx`, `cypress/e2e/menu.cy.ts`. Modify `src/app/layout.tsx`, `src/components/Shortcuts.tsx`, `src/app/globals.css`, `cypress/support/e2e.ts`, `cypress/e2e/keys.cy.ts`, `package.json`, `pnpm-lock.yaml`.

**Interfaces:**
- Produces: `Menu()`, a client component whose state is `open: "shortcuts" | null`; Tasks 2 and 3 widen it and add `approved`.
- Produces: `Shortcuts({ open, onClose }: { open: boolean; onClose: () => void })`, opened by an effect that calls `showModal()` and closed by one that calls `close()`, as `FormsDialog` does.
- Produces: `cy.axe(context?: string)`: fails the test, listing each violation's rule id, impact and selectors, when `axe-core` finds any in `context` (default the whole page) under the tags `wcag2a`, `wcag2aa`, `wcag21aa`, `wcag22aa`. `cy.menuItem(name: string)`: waits for hydration, clicks the menu button, clicks the item containing `name`.
- Produces: `.options-end { position-area: bottom span-left; }`.

- [ ] **Step 1: Add `axe-core`.** `pnpm add -D axe-core` (the workspace has `minimumReleaseAge`; if the newest release is too new, pin the newest allowed one). Write `cy.axe` and `cy.menuItem` in `cypress/support/e2e.ts` with typed declarations: read `node_modules/axe-core/axe.min.js` with `cy.readFile`, evaluate it in the app window, call `axe.run`, assert `violations` is empty with a message built from them. No `any`: type the window as `Window & { axe: typeof import("axe-core") }`.
- [ ] **Step 2: Write the failing tests** in `menu.cy.ts` (describe "the menu"):
  - "opens from the ⋯ button in the header and lists Keyboard shortcuts": visit `/`, hydrate the button, `header [popover]` is not visible, click it, the popover is visible and contains `Keyboard shortcuts`, `cy.axe("header")` passes.
  - "closes on Escape and gives the focus back to the button": open, `cy.pressEscape()`, the popover is gone, `cy.focused()` has `aria-label="Menu"`.
  - for 1920×1100 and 390×844, "keeps the menu inside the window and ends level with its button": left ≥ 0, right ≤ `clientWidth`, `|popover.right − button.right| ≤ 1`, `scrollWidth ≤ clientWidth`.
  - "gives the focus back to the menu button when the keyboard shortcuts close": open the dialog from the item, close it with Close, then again with Escape; both times `cy.focused()` is the menu button.
  - In `keys.cy.ts`, "opens from the ? button in the header…" becomes "opens from the menu, lists each key, and closes with Close" and goes through `cy.menuItem("Keyboard shortcuts")`.
- [ ] **Step 3: Run them to see them fail.** `node scripts/e2e.mjs run --spec cypress/e2e/menu.cy.ts,cypress/e2e/keys.cy.ts`. Expected: the menu tests fail on the missing button, the keys test on the missing `?`. `keys.cy.ts` has a known flake in the full suite (it also fails on the commit before any of this branch's work); judge it by the one test.
- [ ] **Step 4: Implement** `Menu.tsx` as described in Architecture: trigger `className="action min-w-control-sm px-2"` with `popoverTarget` and an anchor name as `Clip.tsx` does; popover `popover="auto" className="options options-end"`; each item `className="options-item"` with `popoverTarget` and `popoverTargetAction="hide"`; one `close()` that sets `open` to `null` and calls `focus()` on the trigger (a `ref`), used as `onClose` of every dialog. Convert `Shortcuts` to the controlled form without its button. In `layout.tsx`, replace the `<nav>` and `<Shortcuts />` with `<Menu />`.
- [ ] **Step 5: Run the same specs again, then `pnpm typecheck`.** Expected: green. If `position-area: bottom span-left` does not end level with the button, the third test fails; fix the CSS, not the test.
- [ ] **Step 6: README** (the `?` and shortcuts mention) and commit: `git add -A src cypress package.json pnpm-lock.yaml README.md docs`.

### Task 2: Backups… and Delete unused audio… move into the menu

**Files:** Modify `src/components/Menu.tsx`, `Backups.tsx`, `UnusedAudio.tsx`, `src/app/page.tsx` (remove the two components and their imports, keep `ExportButton` until Task 3), `cypress/e2e/backups.cy.ts`, `housekeeping.cy.ts`, `menu.cy.ts`, `README.md`.

**Interfaces:**
- Consumes: Task 1's `Menu`, `cy.menuItem`.
- Produces: `Backups({ open, onClose })`, `UnusedAudio({ open, onClose })`. Backups loads its data when `open` becomes true and clears its state when it closes. UnusedAudio runs its check when `open` becomes true and opens its dialog when it has an answer; every way of closing the dialog ends in `onClose`, or the menu could not open it a second time. `Menu`'s state becomes `"shortcuts" | "backups" | "audio" | null`.

- [ ] **Step 1: Change the existing specs first** (they are the failing tests): in `backups.cy.ts` and `housekeeping.cy.ts`, `opener` and `openDialog()` go through `cy.menuItem("Backups")` / `cy.menuItem("Delete unused audio")`; "fits the footer and the dialog at …px" becomes "fits the menu and the dialog at …px" and also checks the open menu is inside the window; the Escape test in `backups.cy.ts` expects the focus on the menu button and opens the dialog again from the menu; housekeeping gets "gives the focus back to the menu button after Cancel and after Close".
- [ ] **Step 2: Add to `menu.cy.ts`:** "has Backups… and Delete unused audio… in the menu and nothing of them at the bottom of the home page": both items are in the open popover, `main` contains neither text.
- [ ] **Step 3: Run `backups.cy.ts`, `housekeeping.cy.ts`, `menu.cy.ts`.** Expected: red, the old buttons are still where they were.
- [ ] **Step 4: Implement** as in Interfaces. Remove each component's own trigger button and `aria-haspopup`; the items carry the text `Backups…` and `Delete unused audio…`.
- [ ] **Step 5: Run the three specs, `pnpm typecheck`, `pnpm test`.** Expected: green.
- [ ] **Step 6: README** (Run/flow item 6 and the Backups paragraph say where the buttons are now) and commit.

### Task 3: Export moves into the menu; the home page loses its footer

**Files:** Modify `src/components/Menu.tsx`, `src/app/layout.tsx`, `src/app/page.tsx`, `src/app/globals.css`, `cypress/e2e/menu.cy.ts`, `README.md`. Delete `src/components/ExportButton.tsx`.

**Interfaces:**
- Consumes: `store.counts().approved` through `context()`.
- Produces: `Menu({ approved }: { approved: number })`; the export action, its disabled state and its message, which the old `ExportButton` held, now live in `Menu`. The message is `<p role="status" className="w-full break-words pb-2 text-sm text-quiet empty:hidden">` as the last child of `.topbar-inner`, which gets `flex-wrap`.

- [ ] **Step 1: Write the failing tests** in `menu.cy.ts` (describe "export from the menu"). Read the count with `cy.request("/api/backups").its("body.check.approved")`.
  - "names the approved cards in its text, and is disabled when there are none": the item's text is `Export <n> approved card(s) to Anki`, or `Export to Anki (nothing approved yet)` and disabled when n is 0.
  - "exports the approved cards, says where the file is and downloads it": approve one card first by the steps of card.cy.ts's "approves the card and goes on to the next word" on a rank no other spec opens (grep `rank=` and `rank:` in `cypress/e2e`); then `cy.menuItem("Export")`; the status reads `Exported <n> card(s) to …/exports/arabic-deck-<date>.apkg`; `cy.readFile` finds that file name under `Cypress.config("downloadsFolder")`.
  - "says why when the export is refused and starts no download": intercept `POST /api/export` with status 400 and `{ error: "…" }`; the message shows that text.
  - "keeps the page from scrolling sideways at 390px with the message showing".
  - "shows the menu on the not-found page and opens it": visit `/cards/nope` with `failOnStatusCode: false`, open the menu.
- [ ] **Step 2: Run `menu.cy.ts`.** Expected: red.
- [ ] **Step 3: Implement.** `layout.tsx` calls `context().store.counts().approved` and passes it. The export item is disabled when `approved === 0`. Use `plural` from `./format` for the message. Remove the footer `<div>` and the imports from `page.tsx` (it uses `count("approved")` elsewhere, keep that); delete `ExportButton.tsx`.
- [ ] **Step 4: Run `menu.cy.ts`, `list.cy.ts`, `layout.cy.ts`, `scale.cy.ts` (the header height is pinned there), `pnpm typecheck`, `pnpm test`.** Expected: green.
- [ ] **Step 5: README** (flow items 5 and 6, the Run section) and commit.

### Task 4: Reset word moves beside the word counter

**Files:** Modify `src/components/CardScreen.tsx`, `Details.tsx`, `cypress/e2e/card.cy.ts`, `README.md`.

**Interfaces:**
- Produces: `Details` loses its `onReset` prop and the reset block (button and explanatory line).
- Produces: in `CardScreen`'s top row, a left group holding the counter and `<button className="action-quiet min-w-control-sm px-2" aria-label="Reset word" title="Reset word" aria-haspopup="dialog" disabled={leaving}>` with the arrow icon; the Saved/Saving status stays on the right.

- [ ] **Step 1: Write the failing tests** in `card.cy.ts` (describe "resetting a word"): `askToReset` clicks `button[aria-label="Reset word"]` and no longer opens Details (this turns the three existing reset tests red); add "puts Reset word right of the word counter with Details closed" (button left ≥ counter right, vertical centres within 8px), "no longer has Reset word inside Details", "gives the focus back to the Reset word button after Cancel", and `cy.axe('[aria-label="Reset word"]')` passes.
- [ ] **Step 2: Run `card.cy.ts`.** Expected: the reset tests fail on the missing button.
- [ ] **Step 3: Implement.** Pass `() => setResetOpen(true)` to the new button; remove `onReset` from `Details` and its call site.
- [ ] **Step 4: Cover a word outside the list.** The "a card made outside the list" describe in `list.cy.ts` shows how a spec reaches such a card; add one test there: its Reset word button is visible, and the dialog says "The word leaves your list; search for it to add it again."
- [ ] **Step 5: Run `card.cy.ts`, `list.cy.ts`, `pnpm typecheck`.** Expected: green. README (flow item 2 says Reset word is under Details) and commit.

### Task 5: Destructive confirmations are red

**Files:** Modify `src/app/globals.css`, `src/components/ResetDialog.tsx`, `UnusedAudio.tsx`, `Backups.tsx`, `cypress/support/e2e.ts`, `cypress/e2e/card.cy.ts`, `housekeeping.cy.ts`, `backups.cy.ts`.

**Interfaces:**
- Produces: `.action-danger`, the `.action-main` rules with `background: rgb(var(--danger))`; implement by listing both selectors in the shared rules, not by copying them.
- Produces: `cy.colorScheme(scheme: "light" | "dark")`, which sets `prefers-color-scheme` through the Chrome DevTools command `Emulation.setEmulatedMedia`, as `pressEscape` uses `Input.dispatchKeyEvent`.

- [ ] **Step 1: Write the failing tests** where each dialog is already opened: in card.cy.ts's "deletes the card…" test, the Reset confirm button's `background-color` is `rgb(193, 63, 58)` and Cancel's is not; in housekeeping's "says how many files…" test, the Delete button's is; in backups' "deletes a backup only after asking…" test the Delete and the Restore confirmations are red. One extra card.cy.ts test under `cy.colorScheme("dark")`: Reset's background is `rgb(229, 115, 115)`, its text `rgb(25, 25, 25)`, and `cy.axe("dialog[open] .action-danger")` passes in both schemes; reset the scheme to light in `afterEach`.
- [ ] **Step 2: Run the three specs.** Expected: red.
- [ ] **Step 3: Implement** the class and use it on the three confirm buttons; Restore uses it too (after review).
- [ ] **Step 4: Run the three specs and `pnpm typecheck`.** Expected: green. README: no change (colour only). Commit.

### Task 6: The home list shows 30 rows a page

**Files:** Modify `src/app/page.tsx`, `cypress/e2e/list.cy.ts`, `cypress/e2e/look.cy.ts`, `README.md`. `src/lib/paging.ts` and its unit test do not change: `paginate` takes the size as an argument.

**Interfaces:**
- Produces: `PAGE_SIZE = 30` in `src/app/page.tsx`, the only place the size lives.

- [ ] **Step 1: Change the existing specs first** (they are the failing tests): in `list.cy.ts`, "shows one page of 100 words" becomes "shows one page of 30 words" with `have.length 30`; "moves between pages with the pager" and "keeps the query when you move between pages" expect 30 rows on page 2; in `look.cy.ts`, "keeps list rows compact" expects 30 rows and an `outerHeight` below 1440 (the same 48px per row as the 4800 it replaces for 100).
- [ ] **Step 2: Run `list.cy.ts` and `look.cy.ts`.** Expected: red, the page still has 100 rows.
- [ ] **Step 3: Implement** `PAGE_SIZE = 30`. README line "`/` every word, 100 a page" becomes 30.
- [ ] **Step 4: Run `list.cy.ts`, `look.cy.ts`, `layout.cy.ts`, `scale.cy.ts`, `card.cy.ts`, `keys.cy.ts`, `housekeeping.cy.ts`, `backups.cy.ts`, `pnpm typecheck`, `pnpm test`.** Expected: green. A spec that looked for a row beyond the 30th on the first page of the default list is changed to find the word by `?q=rank:N`; the page size is not raised to keep an old spec passing.
- [ ] **Step 5: Commit.**

### Task 7: Accessibility and UX sweep (report only)

**Files:** Create `docs/superpowers/reports/2026-11-06-ui-ux-accessibility-sweep.md`. No source file changes; any throwaway scan spec is deleted before the commit.

The owner chose report only, so nothing found here is fixed in this plan. The deliverable is the list, in a form the owner can pick from.

- [ ] **Step 1: Scan with axe-core.** In a throwaway spec (not committed), run `cy.axe()` over the home page (default, with a query, with a sort, last page), a card (no example, with forms, approved), the not-found page, and every dialog and menu state (menu open, Shortcuts, Backups with and without rows, with confirmation, with an error, Delete unused audio in each phase, Forms, Reset). Repeat at 390px and 1280px and in dark mode (`cy.colorScheme("dark")`).
- [ ] **Step 2: Read every file under `src/components`, `src/app` and `globals.css` against a written checklist** and record evidence (file and line, or the axe rule and selector) for each finding: accessible names and Label in Name; roles, states and landmarks; heading outline; keyboard operability, focus order and focus return; live regions (status vs alert); form labels and error association; target size; contrast in both schemes; colour never the only signal; `lang` and `dir` on Arabic; hover-only information (`title`); text resize and reflow at 200%; reduced motion; wording consistency (the "…" rule, button names); destructive actions without confirmation; empty, loading and error states.
- [ ] **Step 3: Verify, do not assume.** These are questions for the sweep, not findings: whether Remove audio in the clip options (which deletes a paid clip) needs a confirmation; whether the example text opening its editor on focus traps or surprises keyboard users; whether the status of a word is understandable on a phone where only its icon shows; whether leaving a card (Reset, Approve) returns to the page of the list it was opened from, since those flows call `listPath(q)` without a page and 30 rows a page makes 167 pages; what the accessibility tree reports for the popover buttons' expanded state; how a word-list row reads to a screen reader; five dialogs repeating the same open/close effect.
- [ ] **Step 4: Write the report:** a table of findings (what, evidence, WCAG criterion or UX principle, severity: Blocker / Serious / Minor / Taste, suggested fix, effort), then "Checked and found fine" so coverage is visible, then "Limits" (no screen reader was run; automated tools cover a minority of problems). Commit the report only. The owner picks what to fix; that becomes a separate, small plan.

---

## Finish

Final review of the whole branch range by a fresh opus reviewer (both skills), one fix pass, then: the owner's list of rulings, deferred minors, the Nix hash instruction if `axe-core` stayed, and `superpowers:finishing-a-development-branch` (merge or PR is the owner's choice; no PR unless asked).
