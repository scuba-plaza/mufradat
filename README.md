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
2. Edit the card where it is shown. Text saves when you leave it or press Enter. Regenerating never overwrites what you typed. The lexical fields (part of speech, gender, root, verb form) are under Details, closed until you open them. The forms are edited in a dialog: Edit forms. Reset word, also under Details, asks first and then deletes the card with its example, forms and audio clip rows, so that the word is back to Not started (an approved card leaves the export, a word you added outside the frequency list leaves the list and is found again by searching for it); the audio files stay in the workspace until Delete unused audio (step 6) removes them.
3. Optionally generate an example (Mistral), and generate or upload audio per clip (Google TTS, MP3 upload). A clip that has audio shows a play button and an options button with Generate again, Upload MP3… and Remove audio; removing a clip takes the card's approval back and leaves the audio file in place until Delete unused audio (step 6). A failure is shown under its text and changes nothing else. The bar at the bottom says what the card still needs.
   Mistral names the word of the card in its sentence and the app marks it, also when it is attached to another word (بِالْقَلَمِ) or has another ending. When the word cannot be told for certain the sentence is kept unmarked and the card says so: select the word and choose Mark target.
4. Approve and next. Editing an approved card takes the approval back.
5. Export to Anki: every approved card, written to `exports/` in the workspace. The same cards always give the same bytes.
6. Now and then, Delete unused audio, a quiet button at the bottom of the home page beside Export. Audio files are named by the hash of their bytes and are never deleted when a clip is removed or replaced, a form is removed or a word is reset, so they pile up. The button counts the files that no clip refers to, asks, and deletes only those; a clip whose text changed since it was made still refers to its file, so that file is kept, and files in `exports/` are not touched.

## The search bar

The bar on the home page searches, filters and sorts the one list. Words you type are looked for in the Arabic and in the English, ignoring vowel marks and the differences between أ إ آ and ا, between ى and ي, and between ة and ه; a word of four letters or more still matches with a typo, and the closest match comes first. When the query has text, the Wiktionary entries that have no card yet are searched with your cards and the frequency words, and show as not started. Terms are separated by spaces and all of them have to hold, at most 12 of them; a comma inside a field means or, at most 12 values.

| Term | Means |
|---|---|
| `kitab`, `"to write"` | text to find; quotes keep a phrase as typed |
| `status:progress,ready` | `todo` (no card), `progress` (the card lacks something), `ready` (nothing lacks, not approved), `approved` |
| `pos:noun,adj` | part of speech |
| `rank:100`, `rank:1-500`, `rank:>2000`, `rank:<=100`, `rank:none` | frequency rank; `none` is a word outside the frequency list: a card you added yourself and, in a text search, a Wiktionary entry |
| `root:كتب`, `gender:f`, `verbform:II` | root (vowels ignored), gender (`m`, `f`, `m+f`), verb form |
| `missing:audio,example` | what the card still lacks: `example`, `translation`, `target`, `audio`; a word with no card lacks all of it |
| `has:error` | a failed example or audio generation is still shown |
| `example:mistral`, `example:mine` | who wrote the example |
| `edited:7d`, `edited:12h`, `edited:2w`, `edited:2026-11-01`, `edited:2026-11-01..`, `edited:..2026-11-01`, `edited:none` | last edit within that time (`h`, `d`, `w`), on a day, since or until a day, or never |
| `-pos:verb` | a minus in front of a field term takes the words out; to find text that starts with a minus, quote it |
| `sort:-edited` | order by `rank`, `edited`, `arabic` or `english`; a minus is descending, words without a value come last |

Without a `sort:` the words come in the order of the frequency list, the ones you added last, and with text the closest match first; only the list with nothing typed puts the cards you are working on at the top. The In progress and Ready tabs add `sort:-edited`.

A term the bar cannot read, or a quote that is not closed, is named under the bar and no words are listed. A card you made from a Wiktionary entry and then gave another headword no longer counts as made from it, so the entry shows again as not started. Clicking Arabic, English or # above the list sorts by it, again for the other way, a third time for the default order; only the `sort:` term changes.

## Layout

One centered column, 44rem wide on small screens and 58rem from 1024px up, where text, controls and spacing are about 1.2 times as large, the size that 120% zoom gives on a 1920×1200 screen at 100% zoom. The sizes are CSS variables in `src/app/globals.css`, set once and overridden once for the larger screens; browser zoom and the system text size still apply.

## Keyboard

On a word, while no field has focus and no dialog is open: Ctrl+← and Ctrl+→ open the previous and the next word of the list the card was opened from, leaving the current one as it is (a card opened from Continue or from the list with nothing typed walks the frequency list by rank, and a card made outside it has no arrows then). A card that has just left the list it was opened from, such as a word you started in the words not started, goes to where it stands in that list, Ctrl+↑ plays the word and Ctrl+↓ plays the example. The list is read again on every press, so in a list sorted by last edit the order changes as you edit. The keys are matched by their position, so they work in any keyboard layout. The ? button in the top bar lists them.

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
