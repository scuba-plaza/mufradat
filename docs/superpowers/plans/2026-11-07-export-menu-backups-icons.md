# Export Fix, Menu Texts, Backups Scroll and Icons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task, inline. The owner asked for no subagents this time. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Exporting writes exactly one file, where the person chooses; the menu reads Export, Backups, Prune audio and Keyboard shortcuts, with a small margin around its button; the Backups dialog scrolls its list so Close is always in view; the app gets favicons and share images.

**Architecture:** `GET /api/export` builds the deck in memory and answers with it as a download, writing nothing; the menu fetches it and hands the bytes to the browser, which asks where to save. The Backups dialog becomes a flex column whose list is the one scrolling part. The icons are static files in `src/app/` that Next's file conventions turn into the `<head>` tags.

**Tech Stack:** Next 15.5.29 route handlers and metadata file conventions, React 19, Tailwind, Vitest, Cypress.

**Spec:** none separate; the decisions are below.

**Skills:** load `anthropic-skills:tdd` and `anthropic-skills:laws-of-simplicity` before each task. Tests go only at the seams confirmed before: route handlers (Vitest) and the browser (Cypress). One failing test at a time, expected values as literals, a mutation proof for each new test. No subagents; the owner will read the result.

**Before Task 1:** branch `claude/funny-hawking-mz7k1n` at `eb55393`, clean tree, `pnpm typecheck`, `pnpm test` (304) and the full Cypress suite (157) green. Run Cypress with `node scripts/e2e.mjs run --spec <files>`; it rebuilds first.

## Decisions

Owner's (from the request and the answers):
1. The menu items read **Export**, **Backups**, **Prune audio**, **Keyboard shortcuts** (no "…", no count). With nothing approved, Export is just disabled, with no reason in its text.
2. The status line says "Preparing the deck…" while the deck is built and "Exported 3 cards." when it is done; a refusal shows its reason.
3. The menu button gets a 4px margin on every side.
4. Exporting: nothing is written to disk until the person chooses where to save, and only one file is written.
5. The Backups dialog does not overflow: its list scrolls, and Close stays fully visible with 30 backups.
6. Favicons and open-graph images are created.

Mine (overrule any of them):
- `GET /api/export` replaces the `POST` that wrote `exports/` and the `GET ?file=` that served it. A deck download changes nothing, so it is a GET, as the backup download is. The `exports/` folder is no longer made or mentioned; one that already exists in a workspace is left alone.
- The menu uses `fetch`, then a Blob and a download link. A plain link would navigate to the JSON of a refusal instead of showing it.
- The card count travels in an `X-Card-Count` response header.
- Icons: a white Arabic letter **م** (first letter of مفردات) in Scheherazade New on the palette's accent blue `#2B7099` (white on it is 5.4:1), as a rounded square for the favicon and a plain square for the Apple icon. The share image is 1200×630: **مفردات** large, "Mufradat" and "Build one Arabic Anki deck, one card at a time." below it. Rendered with Chromium so the Arabic is shaped correctly (Next's `ImageResponse` does not shape Arabic reliably) and committed as PNG and ICO files.
- The page title gets a description for the share preview, and `metadataBase` is taken from the request's host, so the image URLs are right on any port.
- The 4px margin makes the header 4px taller on phones (48px to 52px), because the touch button is 44px high.

## Global Constraints

- No comments in code. Never the `any` type.
- `pnpm typecheck` and `pnpm test` green before every commit; the specs named in the task green; read exit statuses. README updated in the commit that changes behaviour.
- Copy, exact: menu items in this order `Export`, `Backups`, `Prune audio`, `Keyboard shortcuts`; status `Preparing the deck…`, `Exported 3 cards.` (`Exported 1 card.`); refusal text is the server's.
- The download is named `arabic-deck-YYYY-MM-DD.apkg`; the export route writes nothing under the workspace.
- Icons: files under `src/app/` only; no new dependency; the share image alt text is "Mufradat: build one Arabic Anki deck, one card at a time."

## Review Focus

Failure modes the main slices do not reach. Each has its test in the task named.

1. The deck cannot be built (a clip file is missing): the reason shows in the status line, no file is saved and no request repeats (Task 1).
2. One click, one request, one file: no second download, nothing in the workspace (Task 1).
3. 30 backups at a short window, and at a phone's width: Close is in view and neither the dialog nor the page scrolls (Task 3).
4. The head tags resolve: every icon and share URL answers 200 with the right type and size (Task 4).
5. The renamed items still open their dialogs by keyboard and return the focus (Task 2).

## File Structure

Modify: `src/app/api/export/route.ts`, `route.test.ts`; `src/components/Menu.tsx`; `src/components/Backups.tsx`; `src/app/layout.tsx`; `src/app/globals.css` (only if the margin needs it); `cypress/e2e/menu.cy.ts`, `backups.cy.ts`, `housekeeping.cy.ts`, `keys.cy.ts`; `cypress.config.mjs`; `README.md`.

Create: `src/app/favicon.ico`, `icon.png`, `apple-icon.png`, `opengraph-image.png`, `opengraph-image.alt.txt`, `twitter-image.png`; `cypress/e2e/icons.cy.ts`.

---

### Task 1: Export writes nothing until the person saves

**Files:** Modify `src/app/api/export/route.ts`, `src/app/api/export/route.test.ts`, `src/components/Menu.tsx`, `cypress.config.mjs`, `cypress/e2e/menu.cy.ts`, `README.md`.

**Interfaces:**
- Produces: `GET /api/export` answering 200 with the `.apkg` bytes, `Content-Type: application/octet-stream`, `Content-Disposition: attachment; filename="arabic-deck-<date>.apkg"` and `X-Card-Count: <n>`; 400 `{ "error": … }` for "There are no approved cards to export." and for a missing clip, as now. No `POST`, no `?file=`.
- Produces: Cypress tasks `downloads` (names of the `.apkg` files in `downloadsFolder`) and `workspaceExports` (names under `<WORKSPACE>/exports`, `[]` when it does not exist).
- Produces: the Export menu item says `Export` (disabled at zero approved), and `exportDeck()` sets the status to `Preparing the deck…`, fetches `/api/export`, on success saves the Blob through a temporary link named from `Content-Disposition` and sets `Exported <n> card(s).`, on failure sets the response's error text.

- [ ] **Step 1: Write the failing route tests** in `route.test.ts` (GET from `./route`): (a) "answers with the deck as a download and writes nothing to the workspace": status 200, `Content-Disposition` matches `/^attachment; filename="arabic-deck-\d{4}-\d{2}-\d{2}\.apkg"$/`, `X-Card-Count` is `"1"`, the first two body bytes are `0x50 0x4b`, and no file ending `.apkg` and no `exports` folder exist anywhere under the workspace; (b) "refuses when nothing is approved" (400, `There are no approved cards to export.`); keep the missing-clip test on GET.
- [ ] **Step 2: Run `pnpm vitest run src/app/api/export`.** Expected: red (`GET` is the old file server).
- [ ] **Step 3: Implement the route** as in Interfaces; delete the `exports` directory code.
- [ ] **Step 4: Run the route tests.** Expected: green.
- [ ] **Step 5: Write the failing Cypress tests** in `menu.cy.ts` (replacing the old export tests): "exports with one request and one saved file, and writes nothing to the workspace" (approve a card, count `.apkg` files with `cy.task("downloads")`, click Export through `cy.menuItem`, wait for the intercepted `GET /api/export`, `@export.all` has length 1, downloads grew by exactly 1, `workspaceExports` is `[]`); "says Preparing the deck… and then Exported 1 card." (stubbed `GET /api/export` with `X-Card-Count: 1`, a delay and a body); "says why when the export is refused and saves nothing" (stub 400, the message shows, downloads unchanged); the label tests expect the text `Export` (disabled branch on a fresh workspace). Keep the 390px own-row test and the not-found test.
- [ ] **Step 6: Run `menu.cy.ts`.** Expected: red.
- [ ] **Step 7: Implement `Menu.tsx`** and the two tasks in `cypress.config.mjs`; README (flow item 5, the Workspace line and anything that says `exports/`).
- [ ] **Step 8: Run `menu.cy.ts`, `pnpm typecheck`, `pnpm test`; mutation-prove** (the route writes a file again; the menu navigates instead of fetching; a second request) and commit.

### Task 2: Menu texts and the trigger's margin

**Files:** Modify `src/components/Menu.tsx`, `cypress/e2e/menu.cy.ts`, `backups.cy.ts`, `housekeeping.cy.ts`, `keys.cy.ts`, `README.md`.

**Interfaces:** the items are `Export`, `Backups`, `Prune audio`, `Keyboard shortcuts`; the trigger has `className` margin `m-1` (4px, 4.8px from 1024px).

- [ ] **Step 1: Write the failing tests:** in `menu.cy.ts`, "lists Export, Backups, Prune audio and Keyboard shortcuts in that order" (texts of `header [popover] .options-item`, with a card approved) and "puts a 4px margin around the button" (computed margin of each side between 3 and 6px at 1280 and at 390px, and the right edge of the button `margin-right` inside the top bar's content edge; the existing "right end of the top bar" test is changed to subtract that margin). In `backups.cy.ts`, `housekeeping.cy.ts` and `keys.cy.ts`, `cy.menuItem` calls use `Backups`, `Prune audio`.
- [ ] **Step 2: Run the four specs.** Expected: red.
- [ ] **Step 3: Implement:** the three texts, `m-1` on the trigger; README (every `Delete unused audio…` and `Backups…`).
- [ ] **Step 4: Run the four specs, `scale.cy.ts`, `pnpm typecheck`; mutation-prove and commit.**

### Task 3: The Backups dialog scrolls its list

**Files:** Modify `src/components/Backups.tsx`, `cypress/e2e/backups.cy.ts`.

**Interfaces:** the dialog is a flex column with a maximum height; the list (`ol[aria-label="Backups"]`) is `min-h-0 overflow-y-auto`; the heading, the usage lines, the buttons and Close stay outside the scroll.

- [ ] **Step 1: Write the failing test** in `backups.cy.ts`: "keeps Close in view with 30 backups, the list scrolling and nothing above it overflowing" for 1280×500 and 390×600: create backups with `POST /api/backups` until there are 30 rows, open the dialog; Close's rect is inside the window; the dialog's `scrollHeight` is at most its `clientHeight` plus 1; the list's `overflow-y` is `auto` and its `scrollHeight` exceeds its `clientHeight`; the document does not scroll. Delete the backups it made afterwards.
- [ ] **Step 2: Run `backups.cy.ts`.** Expected: red.
- [ ] **Step 3: Implement the layout.** The confirmation view (Restore or Delete) stays as it is.
- [ ] **Step 4: Run `backups.cy.ts`, `pnpm typecheck`; mutation-prove (the list loses `overflow-y-auto`) and commit.**

### Task 4: Favicons and share images

**Files:** Create the icon files listed above and `cypress/e2e/icons.cy.ts`; modify `src/app/layout.tsx`, `README.md`.

**Interfaces:**
- Produces: `src/app/favicon.ico` (16, 32, 48px), `icon.png` (512×512), `apple-icon.png` (180×180, square), `opengraph-image.png` and `twitter-image.png` (1200×630), `opengraph-image.alt.txt`.
- Produces: `layout.tsx` metadata gains `description: "Build one Modern Standard Arabic Anki deck, one card at a time."` and an asynchronous `generateMetadata` whose `metadataBase` is the request's origin (from `headers()`).

- [ ] **Step 1: Write the failing test** in `icons.cy.ts`: visiting `/`, the head has a `link[rel="icon"]`, a `link[rel="apple-touch-icon"]`, `meta[property="og:image"]` (with `og:image:width` 1200 and `height` 630) and `meta[name="twitter:card"]` with `summary_large_image`; each `href` or `content` URL answers 200 with the expected type (`image/png`, `image/x-icon`), and the PNG header (IHDR) of the share image says 1200×630 and of `icon.png` 512×512.
- [ ] **Step 2: Run `icons.cy.ts`.** Expected: red (no tags yet).
- [ ] **Step 3: Make the images** with a Chromium page (Scheherazade New from `public/`), the ICO by writing the three PNGs into an ICO container; place them in `src/app/`. Look at each file to check the Arabic is joined and centred.
- [ ] **Step 4: Implement the metadata** in `layout.tsx` and run `icons.cy.ts`; README (a line about the icons).
- [ ] **Step 5: Run the spec, `pnpm typecheck`, `pnpm build` (no metadataBase warning); mutation-prove (remove one file, change the size) and commit.**

---

## Finish

Whole-suite run in the normal order (typecheck, unit, full Cypress), a read of the whole diff against the owner's six points and the rules (no comments, no `any`), the ledger of rulings, then the branch menu of `superpowers:finishing-a-development-branch` (merge, PR or keep; no PR unless asked).
