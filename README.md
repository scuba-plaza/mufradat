# Mufradat

A local web app for building one Modern Standard Arabic Anki deck, one card at a time.
Nothing is generated or paid for unless you press the button for it.

## Run

    mufradat            # prints http://127.0.0.1:3000; open it yourself
    PORT=3100 mufradat  # another port

From a checkout (Node 24 and the pnpm in `packageManager`): `pnpm install && pnpm build && ./bin/mufradat`.
With Nix: `nix-build -E 'with import <nixpkgs> {}; callPackage ./nix/default.nix {}'`.

## The flow

1. Press Continue on the home page to open the next word that is not approved, pick a word from the list (rank, Arabic, English, status), or search your cards and Wiktionary. Opening a word creates its bare card from Wiktionary; skipping a word puts it at the end of the queue.
2. Edit the card where it is shown. Text saves when you leave it or press Enter. Regenerating never overwrites what you typed. The lexical fields (part of speech, gender, root, verb form) are under Details, closed until you open them. The forms are edited in a dialog: Edit forms.
3. Optionally generate an example (Mistral), and generate or upload audio per clip (Google TTS, MP3 upload). A failure is shown under its text and changes nothing else. The bar at the bottom says what the card still needs.
   Mistral names the word of the card in its sentence and the app marks it, also when it is attached to another word (بِالْقَلَمِ) or has another ending. When the word cannot be told for certain the sentence is kept unmarked and the card says so: select the word and choose Mark target.
4. Approve and next. Editing an approved card takes the approval back.
5. Export to Anki: every approved card, written to `exports/` in the workspace. The same cards always give the same bytes.

## Keyboard

On a word, while no field has focus and no dialog is open: Ctrl+← and Ctrl+→ open the previous and the next word of the frequency list, leaving the current one as it is, Ctrl+↑ plays the word and Ctrl+↓ plays the example. The keys are matched by their position, so they work in any keyboard layout. The ? button in the top bar lists them.

## Pages

- `/` the frequency list, 100 words a page, with the next word on top. It opens on that word's page; `/?page=N` is a page and `/?rank=N` is the page of a rank, with its row marked
- `/next` goes to the card of the next word that is not approved, creating it the first time
- `/cards/<id>` one card
- `/search?q=` your cards and Wiktionary entries
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

The design follows the Notion design system: warm neutrals, soft surfaces, small corners and the system UI font. The colours are CSS variables in `src/app/globals.css` (light, and dark under `prefers-color-scheme`) and the Tailwind names in `tailwind.config.js` point at them, so no component names a colour of its own. Every text, border, focus and fill pair meets WCAG 2.2 AA in both schemes. Controls are 36px high and grow to 44px on touch screens. Arabic uses Scheherazade New (in `public/`, the Anki card uses it too).

The card templates and CSS in `data/template/` are the note type "Arabic MSA Word"; the export test pins them.
