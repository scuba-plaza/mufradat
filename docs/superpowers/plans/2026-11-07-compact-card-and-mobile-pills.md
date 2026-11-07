# A Compact Card and Mobile Pills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans, inline, by one agent, as set out in "How this plan is carried out". Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On a laptop or desktop the card page of a word that has a headword, a hint, a forms section with at least one form, an English meaning, an example and its translation is about 800px tall, so that it fits one screen without scrolling. The forms section, which is the biggest consumer of height, gets much smaller. On a phone the height does not matter and scrolling is fine, but each audio pill becomes a single row placed above its field. On wider screens each pill stays a single column at the right of its field, as it is today.

**Architecture:** Only presentation changes; no data, route or schema changes. `CardScreen.tsx` owns the vertical rhythm of the page, `Clip.tsx` owns the pill and the space reserved for it, and the forms section (a section inside `CardScreen.tsx`, with the read-only `Clip` of each form) is rebuilt as a compact block. The breakpoint that separates "phone" from "wider" is Tailwind's `sm` (640px), the one the card already uses.

**Tech Stack:** Next 15 and React 19 client components, Tailwind 3 with the `--space` token (1.2× from 1024px), Cypress.

## How this plan is carried out

1. **Skill, before every task:** load `anthropic-skills:laws-of-simplicity` with the Skill tool and follow it. Run its checklist before starting a task and again before the code is committed; start from REDUCE (what on this page is not essential, and what can share a row or a line instead of owning one?). The task report names the `simplicity_delta` (-5 to +5) and why. Do not add controls, text or options to make the page shorter; make it shorter by removing and merging.
2. **UI tests are limited to the most crucial aspects of these changes.** The crucial aspects are exactly three, and each gets one test (a short `it`, at most two viewports): (a) on a laptop-sized window the card of a word with forms is no taller than a generous bound around the 800px target, (b) on a phone each pill is one row above its field, (c) on a wider screen each pill is still one column at the right of its field. Everything else is checked by eye and by the existing tests, which are updated only where this change makes them wrong. No new unit tests: nothing here has logic.
3. **One agent implements all tasks.** The same agent (Claude) implements Tasks 1 to 4 in order, committing after each task. No subagent implements anything, and no review happens between tasks.
4. **One fresh review at the end.** When all four tasks are implemented, committed and green, the implementing agent spawns one new review agent with the Agent tool that has none of the implementer's context. Its prompt holds the commit range of the whole round (every task of this plan), this plan's Goal and Decisions, and the Review Focus below; it runs the `code-review` skill at effort level **high** on that range and reports its findings.
5. **Then the implementer fixes all findings.** None is left for later. A finding that does not hold up against the code is answered in the report with the evidence, not skipped silently. The fixes are committed apart ("Fix the findings of the review of the compact card"), then `pnpm typecheck`, `pnpm test` and the full Cypress run are repeated and everything is pushed.

**Before Task 1:** branch `claude/mufradat-deck-filtering-export-9pcyzi` at the latest commit, clean tree; `pnpm install`; `pnpm typecheck`, `pnpm test` and `pnpm e2e` green with their counts written down (the backups test "shows at least three backups at 390x500…" fails on every full run in this workspace and is not part of this work; say so in the report instead of changing it). Run single Cypress specs with `E2E_SKIP_BUILD=1 node scripts/e2e.mjs run --spec cypress/e2e/<file>` after one build.

## Where the height goes today

Measured with Playwright on a fresh workspace, rank 6 (four forms), at 1920×1200 and 1280×800 (identical). Numbers are CSS pixels, from the top of the page.

| Part | Height |
| --- | --- |
| `<main>` as a whole (includes the clearance for the fixed bottom bar) | 903 |
| Headword (`text-ar-xl`) | 131 |
| Hint | 29 |
| Forms section (four forms) | 197 |
| English meaning | 41 |
| Example (editable block) | 86 |
| Example translation | 36 |

The pills also set a floor on wide screens: with 32px buttons the word's block is at least 146px and the example's at least 181px (`Clip.tsx`), and these floors are mostly empty space beside the text. Starting from 903px, the target of about 800px means saving about 100px, and the forms section is where most of it is.

## Decisions

Owner's (from the request):
1. The target is about 800px for the card's `<main>` area on a wider-than-phone window, for a word with a headword, a hint, a forms section with at least one form, an English meaning, an example and a translation. It is a comfort-zone goal, not a cap: more forms, wrapped text or an error message may make the page taller, and nothing may be clipped, hidden or made to scroll inside itself to stay under 800px.
2. The forms section is reduced where possible. This is the main lever.
3. A phone is exempt from the 800px goal and may grow taller. Nothing is shrunk on a phone to save height; the pills must not make the phone layout worse than it is now.
4. On a phone each pill is a single row (all its buttons side by side), above its field, and centred like the field. On wider screens it stays one column at the right of its field, with the same 32px buttons on laptops and 24px below 1024px.
5. Tests only for the crucial aspects (see "How this plan is carried out").

Mine (say so in the report if one turns out wrong; none needs the owner's answer to start):
6. "Phone" is below 640px (`sm`). Between 640px and 1024px the pills stay in a column at the right of the field, at 24px.
7. The 800px target is measured as the bounding box of `<main>` exactly as in the table above, so it includes the clearance reserved for the fixed bottom bar. If the owner meant the content without that clearance, the result will simply be shorter than needed, which is within the comfort goal.
8. The forms section keeps everything it shows today: each form's label, its Arabic, its Play button, and the Edit forms button. Nothing is removed from what a person can see or do; the height comes from layout: the section loses its header row (Edit forms moves into the same row as the forms or becomes an icon in a corner of the panel), each form puts its label, Arabic and Play on one line or in one tight stack, and the paddings and gaps shrink. The section stays centred and keeps wrapping when there are many forms.
9. The reserved minimum heights of the word and example blocks in `Clip.tsx` apply only where the pill is a column (from 640px). On a phone the pill sits in its own row above the field, so no minimum height is needed there, and the reserved row prevents layout shift by existing from the start (it is part of the layout even while there is no audio).
10. If, after the forms section and the spacing have been tightened, the page is still clearly above 800px, the next candidates, in this order, are: the headword's size on wide screens, the gap between the headword block and the English meaning, the gap before the example. The pill floors are not touched: the owner chose their size. If those are not enough, the report says how many pixels remain and from where, and the owner decides.

## Global Constraints

- No comments in code. Never use the TypeScript `any` type.
- Icon buttons keep `title` and have no `aria-label`; SVGs stay `aria-hidden` and `focusable=false`. The groups keep `role="group"` with `aria-label="Audio for …"`, since the tests and keyboard shortcuts rely on them.
- No layout shift: nothing in the card changes size or position when a clip is generated, uploaded, removed, or when a spinner replaces an icon. The `scrollbar-gutter: stable` on the page stays.
- Buttons stay at least 24×24px (WCAG 2.2 AA target size), on a phone too.
- Existing behaviour is unchanged: Play shortcuts (`button[title="Play the word"]`, `button[title="Play the example"]`), tab order (headword, hint, forms, English meaning, example, English translation, with the pill's buttons after the field they belong to or before it as they sit on the page), the readOnly clips of the forms, and the dialogs.
- Update `README.md` where it describes the card and the pills.
- Gates for every task: `pnpm typecheck`, `pnpm test`, and the Cypress specs named in the task are green before committing.

## Task 1: The forms section, compact

**Files:** `src/components/CardScreen.tsx` (the forms section and the read-only `Clip` usage), `src/components/Clip.tsx` (the `readOnly` branch only), `cypress/e2e/layout.cy.ts` and `cypress/e2e/card.cy.ts` only where an existing test breaks.

- [ ] Look at the section at 1280px and 390px before changing it (Playwright screenshot of rank 6 and of a word with a single form, rank 3 with one form added through the API); write down which pixels are padding, header row, label line and Play row.
- [ ] Remove the header row: no `Forms` heading line above the forms (the section keeps its `aria-label="Forms"`, which the tests and screen readers use). Put Edit forms in the same row as the forms, after the last form, as the existing text button, or as a small icon button with `title="Edit forms"` in the panel's top-right corner if that is shorter and still clearly reachable. Pick one and say why in the report.
- [ ] Put each form's label, Arabic and Play button in one compact unit: the label small and quiet beside or above the Arabic as the width allows, the Play button inline with the Arabic rather than on a row of its own under it. The `needsWork` text ("no audio", "outdated") stays visible in the same unit without making it taller than a form without it, and the gap between a label and its Arabic stays at least 4px (an existing test).
- [ ] Shrink the panel's padding and the gaps between forms; keep wrapping and centring.
- [ ] Measure again: the section with four forms should drop from 197px to roughly 100px or less, and with one form to roughly 70px. Record the numbers.
- [ ] Gates: `layout.cy.ts`, `card.cy.ts`, `keys.cy.ts`; update only the tests that this layout makes wrong.
- [ ] Commit: "Make the forms section compact".

## Task 2: The vertical rhythm of the card

**Files:** `src/components/CardScreen.tsx`, `src/components/ExampleText.tsx` and `src/components/Clip.tsx` only if a spacing lives there; `cypress/e2e/layout.cy.ts` only where an existing gap test breaks.

- [ ] Measure `<main>` again at 1920×1200 and 1280×800 with rank 6 (four forms) and with one form; write the numbers down.
- [ ] Tighten the gaps that are larger than they need to be (`mt-6` above the headword block, `mt-5` before the forms and before the English meaning, `mt-10` before the example, the top row, the hint's `mt-2`), in that order, keeping every gap between two text fields at least 8px (the existing "space under a text field" tests) and the groups still clearly separate: the word's group (headword, hint, forms, English meaning) and the example's group.
- [ ] If still clearly above 800px, apply the next candidates from Decision 10, one at a time, measuring after each; stop as soon as `<main>` is about 800px (a few tens of pixels either way is fine).
- [ ] Do not reduce the pill floors in `Clip.tsx` (`lg:min-h-[146px]` for the word and `lg:min-h-[181px]` for the example) and do not make any font smaller than it is on a phone today.
- [ ] Record the final numbers for the table in the report, for 1 form and for 4 forms. If the page stays above 800px, say by how much and from what.
- [ ] Gates: `layout.cy.ts`, `card.cy.ts`, `scale.cy.ts`.
- [ ] Commit: "Tighten the vertical spacing of the card".

## Task 3: One row of pills above the field on a phone

**Files:** `src/components/Clip.tsx`, `cypress/e2e/card.cy.ts` (the existing capsule test and the divider test).

- [ ] Below 640px the group is a horizontal flex row (buttons side by side, with the divider vertical in the example's row, between Remove and New example), centred above the field it belongs to, with `m-1` kept, the same border-state tones (dashed / amber / red) and the same rounded-full shape. From 640px it is the single column at the right of the field, exactly as now, with the 24px buttons below 1024px and 32px from there.
- [ ] Keep it one component: the same markup switches direction with `flex-row sm:flex-col`, the divider with `h-4 w-px sm:h-px sm:w-4` (or the equivalent), and the grid has one column on a phone (`grid-cols-1`) and the three columns from `sm`. The DOM order puts the group before the field on a phone through `order`/grid placement only, so the tab order and the reading order in the markup are unchanged.
- [ ] The reserved minimum heights of the field area apply from `sm` (Decision 9). The row above the field is always present (also with no audio), so nothing shifts when audio appears.
- [ ] Check at 390px and 320px that the example's row (five buttons and the divider, about 160px wide) fits with room to spare and that the field below is not squeezed; check at 640px and 1024px that the column layout returns and that nothing overlaps.
- [ ] Update the existing tests that describe the phone as a column (`keeps the text centred, the capsule in one column…` at 390px, and the divider test, which must use the right axis per width); keep the 24px minimum size assertion.
- [ ] Gates: `card.cy.ts`, `keys.cy.ts`, `layout.cy.ts`, `scale.cy.ts`.
- [ ] Commit: "Put the audio pills in one row above their fields on a phone".

## Task 4: The three crucial tests, README, whole-run check

**Files:** `cypress/e2e/card.cy.ts` (or `layout.cy.ts`, where the card's layout tests live), `README.md`.

- [ ] Test (a): at 1920×1200 and 1280×800, for a word that has forms and an example, `main` is at most 880px tall. The bound is generous on purpose, because the goal is a comfort zone and not a cap; it exists to catch a regression back to 900px and more. Show it red against the code before Task 1 (check out the previous commit's `Clip.tsx`/`CardScreen.tsx` into a scratch copy, or revert in the working tree and restore) and green now.
- [ ] Test (b): at 390px each audio group is a single row (all buttons share one `top`, within 1px) and sits above its field (its bottom is at most the field's top).
- [ ] Test (c): at 1280px each audio group is a single column (all buttons share one `left`, within 1px) at the right of its field (its left is at least the field's right).
- [ ] Remove any older assertions that the new tests make redundant (do not leave two tests for one fact).
- [ ] Update `README.md`: the card paragraph (compact forms; the pills are one row above the field on a phone and one column at the right of the field from 640px).
- [ ] Run the whole gate once: `pnpm typecheck`, `pnpm test`, and the full Cypress run. Everything is green apart from the known backups test.
- [ ] Commit: "Test the compact card and the pill layouts, and update the README".

## Review (after all four tasks)

As set out in "How this plan is carried out": one fresh review agent, effort level `high`, over the commit range of Tasks 1 to 4 together; then the implementer fixes all findings and pushes.

**Review Focus**
- Anything that makes the card taller, shorter or differently positioned when a clip is generated, uploaded or removed, or when a message appears (layout shift), at 1920, 1280, 1024, 640, 390 and 320px.
- Overlaps or clipping: pills against fields, the forms section against its neighbours, text against the fixed bottom bar, at the same widths and with long forms, many forms, a long hint and a two-line message.
- Tab order, reading order and screen reader names against the markup before this round (visual reordering by `order` must not change the DOM order); `title`-only buttons; 24px minimum target size.
- Tests: are the three crucial tests deterministic, and do the updated older tests still check what they were written for? Are there redundant tests left, or tests that only repeat the implementation?
- Dead code: Tailwind classes or tokens that no longer have any effect, unused props.
- Simplicity: is there anything in the forms section or the pill that can still be removed or shared?

## Finish

- `git log` shows one commit per task and one for the review's fixes; the branch is pushed.
- Final report to the owner: the table of heights (before and after, for one form and for four forms, at 1920×1200 and 1280×800), before and after screenshots at 1280px and 390px, how far the page is from 800px if it is not there, which of Decision 10's levers were used, the `simplicity_delta` of each task, the findings of the review and what was done with each, and the known backups failure.
