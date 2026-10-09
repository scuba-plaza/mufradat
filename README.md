# mufradat

Build an [Anki](https://apps.ankiweb.net/) deck of the most common Modern
Standard Arabic words, from any directory on a Linux machine.

- **Words in frequency order.** The 5000 most useful lemmas, ranked from
  subtitles (everyday speech) and written MSA, with greetings, numbers and days
  pulled up early.
- **Cards written by Claude.** `add` asks [Claude Code](https://claude.com/claude-code)
  for the meaning, a hint where one is needed, the forms and a fully vowelled
  example sentence built from common words. It uses your own Claude login, so a
  Pro or Max plan is enough and no API key is needed.
- **Vowels checked twice.** [CAMeL Tools](https://github.com/CAMeL-Lab/camel_tools)
  and [CATT](https://github.com/abjadai/catt) read every word independently; a
  review page in your browser shows where they disagree with the card.
- **Audio.** Google Cloud Text-to-Speech reads the word, its forms and the
  sentence. Clips are named after the voice, so switching voices and back finds
  the old ones again.
- **Your data stays yours.** Notes, decisions, audio and the built deck live in a
  workspace directory that you choose. The tool only holds code and the shipped
  word list.

Cards are `Recognition` (Arabic to English) for every word, plus `Production`
(English to Arabic) for the first 1000. Each note is tagged with its part of
speech, a rank bucket and its check state.

## Install

You need [Claude Code](https://claude.com/claude-code), signed in, to write
notes, and a Google Cloud project with the Text-to-Speech API enabled to make
audio. Vowel checking is optional and needs Python 3.11 or newer.

With Nix (recommended on NixOS, because it sets up the C++ libraries the vowel
checkers need):

```sh
nix run github:scuba-plaza/mufradat -- --help
nix profile install github:scuba-plaza/mufradat
```

From a release (Linux, amd64 or arm64): unpack the tarball from the
[releases page](https://github.com/scuba-plaza/mufradat/releases) and put
`mufradat` on your `PATH`. Shell completions are in `completions/`.

With Go:

```sh
go install github.com/scuba-plaza/mufradat/cmd/mufradat@latest
```

## Quick start

```sh
mufradat init ~/arabic          # a workspace for your notes and audio
cd ~/arabic
mufradat auth google --import ~/key.json    # Google service-account key
mufradat setup                  # optional: the vowel checkers, about 2 minutes
mufradat add 25                 # the 25 most common words
mufradat check                  # cross-check the vowels
mufradat review                 # fix what the checkers doubt, in your browser
mufradat audio                  # make the clips
mufradat build                  # out/*.apkg, ready to import into Anki
```

`mufradat status` shows where the workspace stands and prints the exact
command to run next. Every command finds the workspace from the current
directory, from `-C DIR`, or from `MUFRADAT_WORKSPACE`; without any of
those it uses the default under `~/.local/share/mufradat/default`.

Import the `.apkg` with *File > Import* in Anki. The deck and note type have
their own identity, so importing a newer build updates the cards in place.

To add particular words instead of the next most common ones:

```sh
mufradat add كتاب عين
mufradat add ماء --entry ماء#2      # the verb "to meow", not the noun "water"
mufradat add -f words.txt
```

A spelling can stand for several words. On a terminal `add` asks which one you
mean; elsewhere pass `--entry WORD#N`, `--pos`, `--pick N` or `--yes`.

## Commands

| Command | What it does |
|---|---|
| `init [dir] [--git]` | Create a workspace. |
| `add [N \| WORD...]` | Write the next N most common words, or the words you name. Every finished note is saved at once, so an interrupted run loses nothing. |
| `check [--strict]` | Cross-check the vowels. Results are remembered; only new or changed text is checked. `--strict` exits 4 while major flags remain. |
| `review [--all]` | Go through flagged notes (or every note) in your browser: accept, add a missing ending, edit, ask Claude for a better version, hear the clips, remake or remove a clip, undo. |
| `audio [--dry-run]` | Make the clips that are missing. `audio gc` lists clips no note needs with the current voice, `--delete` removes them. |
| `build [-o PATH]` | Write the `.apkg`. The same inputs give identical bytes. |
| `voices list`, `voices compare` | List Google's Arabic voices, or listen to them side by side and pick one. |
| `setup` | Install the vowel checkers (CAMeL Tools and CATT). |
| `auth google --import FILE` | Keep a Google service-account key in the user config directory. |
| `config get`, `config set`, `config path` | Show or change settings. |
| `status`, `doctor` | Where things stand, and whether everything the tool needs is in place. |
| `licenses`, `version`, `completion` | The licences of the code, data and font; the version; shell completions. |

Global flags: `-C/--workspace`, `-v`, `-q`, `--no-color`. `status`, `doctor`,
`check` and `add` take `--json`. Data goes to standard output and progress and
diagnostics to standard error. `NO_COLOR` is honoured.

Exit codes: `0` success, `1` failure, `2` wrong usage, `3` not set up,
`4` `check --strict` found major flags, `75` temporary (your Claude plan's usage
limit or a busy service; run the command again later), `130` interrupted.

## How the cards are checked

`check` reads every Arabic word of a note with two independent tools:

1. CAMeL Tools' morphological analyser must accept the card's vowelling.
2. In sentences, CATT vowels the bare text and must agree, with CAMeL's
   contextual model as a second opinion.
3. A letter either tool reads without a vowel, such as a missing case ending, is
   flagged.

The build tags what is left: `check::diacritics` when no independent source
supports the vowels, `check::diacritics-minor` when only CATT disagrees and
CAMeL agrees with the card, and `check::unverified` when `setup` has not been
run. Without `setup`, `add`, `audio` and `build` still work.

`add` keeps example sentences simple: the sentence may only use words the learner
already knows, that is words up to `author.allow_rank` (default 1500) or up to
the card's own rank if that is higher. A sentence with rarer words goes back to
Claude once with the offending words named. Set `author.allow_rank` to 0 to turn
this off.

## Settings

Settings come from built-in defaults, then `~/.config/mufradat/config.toml`,
then the workspace's `mufradat.toml`. `mufradat config get` lists them.

| Key | Default | |
|---|---|---|
| `deck_name` | `Arabic MSA Core` | Name of the Anki deck. |
| `anki.production_top` | `1000` | Words up to this rank also get a Production card. |
| `audio.voice`, `audio.rate` | `ar-XA-Chirp3-HD-Achernar`, `0.9` | The voice and speaking rate. Changing them never reuses another voice's clips. |
| `author.model`, `author.effort` | `sonnet`, `medium` | Passed to Claude Code. |
| `author.batch_size`, `author.concurrency` | `10`, `2` | Words per request and requests at once. |
| `author.allow_rank` | `1500` | The vocabulary the example sentences may use. `0` turns the check off. |
| `tts.requests_per_minute`, `tts.concurrency` | `300`, `4` | How hard to push Google. |
| `tts.quota_project`, `tts.endpoint` | empty | For user credentials that need a billing project, or a regional endpoint. |
| `verify.enabled` | `auto` | `on` makes a missing checker an error, `off` skips checking. |
| `workspace`, `google.credentials_file`, `verify.python` | empty | User config only: the default workspace, a credentials file, and your own Python with the checkers. |

Google credentials are looked for in this order: `GOOGLE_APPLICATION_CREDENTIALS`,
`google.credentials_file`, the key imported with `auth google --import`, then
gcloud's application default credentials.

## Anki settings

Suggested deck options for `Arabic MSA Core`:

- **FSRS** on, desired retention 0.90.
- **New cards/day** 15 to 20. Each of the first 1000 words has two cards, so
  that is about ten new words a day.
- **Maximum reviews/day** high enough that reviews never pile up (200 or more).
- **Bury new siblings** and **bury review siblings** on, so a word's two cards
  do not appear on the same day.
- **New card gather order** "Deck" and **sort order** "Order gathered", to keep
  the frequency order.
- **Leech action** "Tag only", to find the words that need a better sentence.

## More

- [`docs/workspace.md`](docs/workspace.md): what is in a workspace and which
  files are safe to edit or commit.
- [`docs/architecture.md`](docs/architecture.md): how the pieces fit together.
- [`docs/troubleshooting.md`](docs/troubleshooting.md): NixOS, usage limits,
  credentials, slow first runs.
- [`data/SOURCES.md`](data/SOURCES.md): where the word list comes from, its
  licences, and how to rebuild it.
- [`CONTRIBUTING.md`](CONTRIBUTING.md): building, testing and the project's rules.

## Licence

The code is MIT (`LICENSE`). The word list and dictionary data are derived from
Wiktionary and two frequency lists and are CC BY-SA 4.0 (`NOTICE`,
`data/SOURCES.md`). The bundled font, Scheherazade New by SIL, is under the SIL
Open Font License. `mufradat licenses` prints all of them.
