# Mufradat

A local web app for building one Modern Standard Arabic Anki deck, one card at a time.
Nothing is generated or paid for unless you press the button for it.

## Run

    mufradat            # prints http://127.0.0.1:3000; open it yourself
    PORT=3100 mufradat  # another port

From a checkout (Node 24 and the pnpm in `packageManager`): `pnpm install && pnpm build && ./bin/mufradat`.
With Nix: `nix build`; the flake's `overlays.default` adds `mufradat` to nixpkgs.

## The flow

1. Press Continue on the home page to pick up where you left off: the card you edited last that is not approved, or, when none is left, the most frequent word you have not started. You can also pick a word from the list or find it with the search bar. Opening a word creates its bare card from Wiktionary.
2. Edit the card where it is shown. Text saves when you leave it or press Enter. Regenerating never overwrites what you typed. The lexical fields (part of speech, gender, root, verb form) are under Details, closed until you open them. The forms are edited in a dialog: Edit forms.
3. Optionally generate an example (Mistral), and generate or upload audio per clip (Google TTS, MP3 upload). A clip that has audio shows a play button and an options button with Generate again, Upload MP3… and Remove audio; removing a clip takes the card's approval back and leaves the audio file in place. A failure is shown under its text and changes nothing else. The bar at the bottom says what the card still needs.
   Mistral names the word of the card in its sentence and the app marks it, also when it is attached to another word (بِالْقَلَمِ) or has another ending. When the word cannot be told for certain the sentence is kept unmarked and the card says so: select the word and choose Mark target.
4. Approve and next. Editing an approved card takes the approval back.
5. Export to Anki: every approved card, written to `exports/` in the workspace. The same cards always give the same bytes.

## The search bar

The bar on the home page searches, filters and sorts the one list. Words you type are looked for in the Arabic and in the English, ignoring vowel marks and hamza, ya and ha variants, and a word of four letters or more still matches with a typo; the closest match comes first. Your cards, the frequency words and the Wiktionary entries that have no card yet are searched together, and the entries without a card show as not started. Terms are separated by spaces and all of them have to hold; a comma inside a field means or.

| Term | Means |
|---|---|
| `kitab`, `"to write"` | text to find; quotes keep a phrase as typed |
| `status:progress,ready` | `todo` (no card), `progress` (the card lacks something), `ready` (nothing lacks, not approved), `approved` |
| `pos:noun,adj` | part of speech |
| `rank:100`, `rank:1-500`, `rank:>2000`, `rank:<=100`, `rank:none` | frequency rank; `none` is a word you added yourself |
| `root:كتب`, `gender:f`, `verbform:II` | root (vowels ignored), gender (`m`, `f`, `m+f`), verb form |
| `missing:audio,example` | what the card still lacks: `example`, `translation`, `target`, `audio`; a word with no card lacks all of it |
| `has:error` | a failed example or audio generation is still shown |
| `example:mistral`, `example:mine` | who wrote the example |
| `edited:7d`, `edited:12h`, `edited:2026-11-01..`, `edited:..2026-11-01`, `edited:none` | last edit within that time, since or until a day, or never |
| `-pos:verb` | a minus in front of a field term takes the words out |
| `sort:-edited` | order by `rank`, `edited`, `arabic` or `english`; a minus is descending, words without a value come last |

Without a `sort:` the words come in the order of the frequency list, the ones you added last, and with text the closest match first; only the list with nothing typed puts the cards you are working on at the top. The In progress and Ready tabs add `sort:-edited`.

A term the bar cannot read is named under it and no words are listed. Clicking Arabic, English or # above the list sorts by it, again for the other way, a third time for the default order; only the `sort:` term changes.

## Layout

One centered column, 44rem wide on small screens and 58rem from 1024px up, where text, controls and spacing are about 1.2 times as large, the size that 120% zoom gives on a 1920×1200 screen at 100% zoom. The sizes are CSS variables in `src/app/globals.css`, set once and overridden once for the larger screens; browser zoom and the system text size still apply.

## Keyboard

On a word, while no field has focus and no dialog is open: Ctrl+← and Ctrl+→ open the previous and the next word of the list the card was opened from, leaving the current one as it is (a card opened from Continue or from the list with nothing typed walks the frequency list by rank), Ctrl+↑ plays the word and Ctrl+↓ plays the example. The list is read again on every press, so in a list sorted by last edit the order changes as you edit. The keys are matched by their position, so they work in any keyboard layout. The ? button in the top bar lists them.

## Pages

- `/` every word, 100 a page, found with the search bar. With nothing typed: the cards you are working on first (latest edit on top, "in progress" or "ready"), then the words you have not started by frequency, then the approved ones. The tabs above the list show how many words are in each state and filter by it. A card made outside the frequency list shows a dash for its rank. `/?q=…&page=N` is a query and a page
- `/next` goes to the card Continue opens, creating it the first time
- `/cards/<id>` one card
- `/open?rank=N` and `/open?lemma=…&pos=…` go to a word's card, creating it the first time

## Files

- Workspace (`~/.local/share/mufradat`, or `MUFRADAT_WORKSPACE`): `vocabulary.db`, `audio/` (named by the hash of the audio), `exports/`.
- Config `~/.config/mufradat/config.toml`, readable only by you (`chmod 600`); the app refuses to read it otherwise:

      mistral_api_key = "..."
      google_credentials_file = "~/.config/mufradat/google-credentials.json"  # service account or application default credentials
      audio_rate = 0.9
      # mistral_model = "mistral-medium-latest"
      # mistral_reasoning_effort = "high"  # the model thinks before it answers: better grammar, but thousands of tokens and many seconds per example
      # tts_voice = "ar-XA-Chirp3-HD-Achernar"
      # deck_name = "Arabic MSA Core"
      # deck_id = 1700000000001
      # model_id = 1700000000002

  `MISTRAL_API_KEY` and `GOOGLE_APPLICATION_CREDENTIALS` override the file.
  To update a deck already in Anki, set `deck_id` and `model_id` to the values it was first exported with.

## Development

    pnpm test        # vitest: card store, approval rules, paging, Anki export, embedded data
    pnpm typecheck   # tsc --noEmit, for the app and for the Cypress specs
    pnpm e2e         # Cypress against a production build; never part of pnpm test
    pnpm e2e:open    # the same, in the interactive runner

`pnpm e2e` builds the app, starts it on port 3100 (`E2E_PORT`) with an empty temporary workspace and no API keys, so it cannot touch your cards or spend credits, and removes everything afterwards. `E2E_SKIP_BUILD=1` reuses the last build, and Cypress options pass through: `pnpm e2e --spec cypress/e2e/list.cy.ts`.
The specs run in one browser everywhere, locally and in CI: the Chrome for Testing version pinned in `cypress/browser.json`, which `pnpm e2e` downloads once into `~/.cache/mufradat/browsers`. To move to a newer Chrome, change that version; both places follow.
Cypress downloads its own binary on `pnpm install`; on NixOS that and the downloaded Chrome need `nix-ld`. Set `CYPRESS_INSTALL_BINARY=0` to skip the download when only the unit tests are needed.

The design follows the Notion design system: warm neutrals, soft surfaces, small corners and the system UI font. The colours are CSS variables in `src/app/globals.css` (light, and dark under `prefers-color-scheme`) and the Tailwind names in `tailwind.config.js` point at them, so no component names a colour of its own. Every text, border, focus and fill pair meets WCAG 2.2 AA in both schemes. Controls are 36px high, and 44px from 1024px up and on touch screens. Arabic uses Scheherazade New (in `public/`, the Anki card uses it too).

The card templates and CSS in `data/template/` are the note type "Arabic MSA Word"; the export test pins them.
