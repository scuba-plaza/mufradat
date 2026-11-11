# Mufradat

A local web app for building your Modern Standard Arabic Anki deck, one card at a time.
Nothing is generated or paid for unless you press the button for it.

## Run

    mufradat            # prints http://127.0.0.1:3000; open it yourself
    PORT=3100 mufradat  # another port

From a checkout (Node 24 and the pnpm in `packageManager`): `pnpm install && pnpm build && ./bin/mufradat`.
With Nix: `nix build`; the flake's `overlays.default` adds `mufradat` to nixpkgs.

## The flow

1. Press Continue on the home page to pick up where you left off: the card you edited last that is not approved, or, when none is left, the most frequent word you have not started. You can also pick a word from the list or find it with the search bar. Opening a word creates its bare card from Wiktionary.
2. Edit the card where it is shown. Text saves when you leave it or press Enter. Each side of the card has its own hint in quiet text under it, the Arabic hint under the headword and the English hint under the English meaning; in the deck the Arabic hint is on the front of the recognition card, the English hint on the front of the production card and the back of the recognition card. Regenerating never overwrites what you typed. Tags and the lexical fields (part of speech, gender, root, verb form) are not on the card page: the tag icon right of Reset word opens the Tags and details dialog. A tag has no spaces (typing Lesson 1 stores lesson_1), is saved when you press Enter or leave the field, and does not take an approval back. The forms sit in one compact panel under the headword, whose border shows the state of their audio (dashed when a form has none, amber when one is out of date, red on a failure), and the pencil icon at its right opens the dialog where they are edited; with no forms the panel is dashed, says No forms, and shows a plus icon instead. Reset word, the ↺ button right of the word counter at the top of the card, asks first, and then deletes the card with its example, forms, tags and audio clip rows, so that the word is back to Not started (an approved card leaves the export, a word you added outside the frequency list leaves the list and is found again by searching for it); the audio files stay in the workspace until Prune audio (step 6) removes them. Suspend word, the pause button right of the ↺ button, parks the card: it takes the approval back, the word is never exported, and no search finds it but status:suspended; it asks first when the card is approved. The same button, then a play icon, brings the word back, and Reset word brings it back unsuspended.
3. Optionally generate an example (a Mistral agent), and generate or upload audio per clip (Google TTS, MP3 upload). The word and the example each have a capsule of icon buttons at the right of their field: Play (it shows that the clip is playing and is disabled meanwhile; a clip you generate or upload plays by itself, after the one that is playing, if there is one), Generate (Regenerate once there is audio), Upload and Remove, and on the example also New example, set apart by a divider. The capsule holds icon buttons of 24px (32px from 1024px wide): one row above its field below 640px, and one column at the right of its field from 640px, where the field is kept at least as tall so nothing moves; the text stays centred on the page. Its border shows the state of the audio: dashed when there is none, amber when the Arabic changed since the audio was made, red while a failure is shown. While something is generated, uploaded or imported, a spinner replaces the icon of the play button (or of the Export icon, or the label of the button), so nothing on the page moves. Removing a clip takes the card's approval back and leaves the audio file in place until Prune audio (step 6). A failure is shown under its text and changes nothing else. The bar at the bottom says what the card still needs.
   The app calls the agent set by `mistral_agent_id` and `mistral_agent_version` in the config (the Files section below), so the Mistral API key must belong to the workspace that owns it; the agent holds the instructions, model and sampling settings. The app sends the fields of the card that are not empty as a JSON object (`headword`, `pos`, `english`, `gender`, `root`, `verb_form`, `forms`, `hint`, `hint_ar`, and the `current_example` when the card has one, so that the agent writes another sentence); a card with no English meaning gets no request, only an error. The agent answers with a JSON object of `sentence` and `translation`, which the card keeps. The app saves the answer only if the sentence has 2 to 12 words, is Arabic, has no square brackets and has a vowel mark on every word, and the translation is not Arabic (the rules of the agent's prompt that code can check). The agent does not name the word of the card, so the app marks the headword or a form of it as it does for a typed example (next paragraph); when the word is not in the sentence or stands twice, the sentence is kept unmarked: select the letters you are teaching.
   An example you type is marked the same way when you leave the field, looking for the headword and then the forms (also with a preposition, conjunction or pronoun attached, which are left out of the mark). While you edit, selecting letters marks exactly those letters instead, part of a word included when you leave the field: the letters selected last are marked, so you can adjust the selection for as long as you like, and to take a mark away you delete its brackets in the field (only a sentence you newly type is marked by itself). There is no button and nothing to type; the brackets in the field are the same mark and may be typed.
4. Approve (it opens the next word). Editing an approved card takes the approval back.
5. Export, the download icon right of the Search button on the home page (hover it to read how many cards it exports): the approved cards of the search last made, as one Anki package, handed to your browser to save where you choose. With nothing searched that is every approved card; with `tag:lesson_*` it is the approved cards of those lessons, all of them and not just the page shown. The icon is disabled when the search finds no approved card, and a search the bar cannot read is named under the bar. Text typed in the bar changes nothing until it is searched. Each card's tags go into the package as Anki tags, after its pos:: and rank:: tags. Nothing is written to the workspace. The same cards always give the same bytes. Anki updates a note it already has only when the card was edited, and tagging does not count as an edit, so changing a tag on a card that is already in a deck does not reach that deck by importing again.
6. Now and then, Prune audio, in the menu (the ⋯ button at the right of the top bar). Audio files are named by the hash of their bytes and are never deleted when a clip is removed or replaced, a form is removed or a word is reset, so they pile up. It counts the files that no clip refers to, asks, and deletes only those; a clip whose text changed since it was made still refers to its file, so that file is kept.

## The search bar

The bar on the home page searches, filters and sorts the one list. Words you type are looked for in the Arabic and in the English, ignoring vowel marks and the differences between أ إ آ and ا, between ى and ي, and between ة and ه; a word of four letters or more still matches with a typo, and the closest match comes first. When the query has text, the Wiktionary entries that have no card yet are searched with your cards and the frequency words, and show as not started. Terms are separated by spaces and all of them have to hold, at most 12 of them; a comma inside a field means or, at most 12 values.

| Term | Means |
|---|---|
| `kitab`, `"to write"` | text to find; quotes keep a phrase as typed |
| `status:progress,ready` | `todo` (no card), `progress` (the card lacks something), `ready` (nothing lacks, not approved), `approved`, `suspended` (parked with Suspend word, and found by no search but this one) |
| `pos:noun,adj` | part of speech |
| `rank:100`, `rank:1-500`, `rank:>2000`, `rank:<=100`, `rank:none` | frequency rank; `none` is a word outside the frequency list: a card you added yourself and, in a text search, a Wiktionary entry |
| `root:كتب`, `gender:f`, `verbform:II` | root (vowels ignored), gender (`m`, `f`, `m+f`), verb form |
| `missing:audio,example` | what the card still lacks: `example`, `translation`, `target`, `audio`; a word with no card lacks all of it |
| `has:error` | a failed example or audio generation is still shown |
| `example:mistral`, `example:mine` | who wrote the example |
| `edited:7d`, `edited:12h`, `edited:2w`, `edited:2026-11-01`, `edited:2026-11-01..`, `edited:..2026-11-01`, `edited:none` | last edit within that time (`h`, `d`, `w`), on a day, since or until a day, or never |
| `tag:lesson_1`, `tag:lesson_*`, `-tag:*` | cards with that tag, ignoring case; `*` stands for any run of characters (every other character is taken as typed), so `lesson_*` finds `lesson_1` and `lesson_10`; `-tag:*` finds the words with no tag |
| `-pos:verb` | a minus in front of a field term takes the words out; to find text that starts with a minus, quote it |
| `sort:-edited` | order by `rank`, `edited`, `arabic` or `english`; a minus is descending, words without a value come last |

The download icon right of Search exports the approved cards of the search last made (step 5 of the flow above).

Without a `sort:` the words come in the order of the frequency list, the ones you added last, and with text the closest match first; only the list with nothing typed puts the cards you are working on at the top. The In progress and Ready tabs add `sort:-edited`.

A term the bar cannot read, or a quote that is not closed, is named under the bar and no words are listed. A card you made from a Wiktionary entry and then gave another headword no longer counts as made from it, so the entry shows again as not started. Clicking Arabic, English or # above the list sorts by it, again for the other way, a third time for the default order; only the `sort:` term changes.

## Backups

Backups in the menu (the ⋯ button at the right of the top bar) opens the list of backups. A backup is the whole workspace at one moment: the database and every audio clip it needs, so a restore never needs audio generated again.

- Taken automatically, with no setting: once a day, on the first visit of the day that finds cards. Nothing else takes one on its own. Back up now takes one when you want it.
- Kept: the newest 14 daily backups, and at most 1 GiB for all of them together; a clip that several backups share counts once. The newest daily backup is kept even when it alone is over the limit. Backups made with Back up now, and imported ones, are never removed on their own and do not count toward the limit. A Mufradat before this one also took backups just before a reset or a restore; those it left behind are kept and removed the same way.
- Restore brings the workspace back to a backup, and ends on the home page. Nothing is saved of the state it replaces, so make a backup first if you may want the current state back. A backup that is damaged, written by a newer Mufradat, or missing audio is refused and nothing changes. Delete… asks first and cannot be undone.
- The dialog also names any clip whose audio file is missing from the workspace, with a link to its card, and any problem the database reports about itself.
- Download saves a backup as one zip file with the database, a manifest and every clip, so a copy can live somewhere other than this computer. The dialog says when you last downloaded one; a download that is cancelled part way does not count.
- Import backup… takes such a zip, checks it (it must be a whole, consistent backup of this app, from this or an older version) and adds it to the list as an Imported backup; it changes nothing by itself. Restore then brings the workspace to it, which is how a new computer or an empty workspace gets the cards and the audio back. A backup can be hundreds of megabytes; the limit is 3 GiB.
- Prune audio never touches a backup's own copies of the audio.
- These backups are on this computer. Copying the workspace folder copies them too.

## Layout

One centered column, 44rem wide on small screens and 58rem from 1024px up, where text, controls and spacing are about 1.2 times as large, the size that 120% zoom gives on a 1920×1200 screen at 100% zoom. The sizes are CSS variables in `src/app/globals.css`, set once and overridden once for the larger screens; browser zoom and the system text size still apply.

## Keyboard

On a word, while no field has focus and no dialog is open: Ctrl+← and Ctrl+→ open the previous and the next word of the list the card was opened from, leaving the current one as it is (a card opened from Continue or from the list with nothing typed walks the frequency list by rank, and a card made outside it has no arrows then). A card that has just left the list it was opened from, such as a word you started in the words not started, goes to where it stands in that list, Ctrl+↑ plays the word and Ctrl+↓ plays the example (nothing happens while that clip is already playing). The list is read again on every press, so in a list sorted by last edit the order changes as you edit. The keys are matched by their position, so they work in any keyboard layout. Keyboard shortcuts in the menu (the ⋯ button at the right of the top bar) lists them.

## Pages

- `/` every word, 30 a page, found with the search bar. With nothing typed: the cards you are working on first (latest edit on top, "in progress" or "ready"), then the words you have not started by frequency, then the approved ones. The tabs above the list show how many words are in each state and filter by it. A card made outside the frequency list shows a dash for its rank. `/?q=…&page=N` is a query and a page
- `/next` goes to the card Continue opens, creating it the first time
- `/cards/<id>` one card
- `/open?rank=N` and `/open?lemma=…&pos=…` go to a word's card, creating it the first time

## Files

- Workspace (`~/.local/share/mufradat`, or `MUFRADAT_WORKSPACE`): `vocabulary.db`, `audio/` (named by the hash of the audio), `snapshots/` (the backups, with their own copy of the audio).
- Icons and share image: `src/app/icon.svg`, `favicon.ico`, `apple-icon.png`, `opengraph-image.png` (Next uses it for Twitter cards too): a stack of flashcards, the letter م in Scheherazade New Bold on a highlighter stripe, on a dark tile. They are static files; Next adds their tags to every page.
- Config `~/.config/mufradat/config.toml`, readable only by you (`chmod 600`); the app refuses to read it otherwise:

      mistral_api_key = "..."
      mistral_agent_id = "ag_..."  # the Mistral agent that writes the examples
      mistral_agent_version = 1  # its version; there is no default for either, a card shows the error until both are set
      google_credentials_file = "~/.config/mufradat/google-credentials.json"  # service account or application default credentials
      audio_rate = 0.9
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
