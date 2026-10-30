# Architecture

mufradat is a single Go binary plus a small Python sidecar for vowel
checking. The binary embeds the word list, the dictionary, the card templates,
the font and the sidecar's source; nothing is read from the installation
directory at run time.

## Layers

```
cmd/mufradat        main: signal handling and os.Exit, nothing else
internal/cli            cobra commands: flags, output, exit codes
internal/app            the composition root: builds every service from env.Env
internal/author         add: target selection, batching, gates, saving
internal/mistral, llm   the Mistral client and chat backend, error classes, retries, usage log
internal/prompt         the guide, the examples, the JSON schema, answer parsing
internal/vocab          the vocabulary gate for example sentences
internal/verify         readings cache and the issues derived from them
internal/sidecar        the sidecar protocol client
internal/pyenv          setup, doctor and launch of the Python environment
internal/review         the review web server and its embedded page
internal/clips, tts     clip naming and planning, the Speaker interface; the Google REST client
internal/httpretry      the retry transport and Retry-After parsing the API clients use
internal/deck, apkg     note type, templates and the deterministic .apkg writer
internal/store          notes.jsonl and decisions.jsonl under a lock
internal/note           the note model, ids, example markup, validation
internal/lexicon        the embedded dictionary and the sense lookup
internal/ranking        the embedded ranked list
internal/morph          clitic-aware matching of spellings to lemmas
internal/tashkeel       Arabic diacritics: skeletons, comparison, ending marks
internal/workspace      discovery, layout, lock
internal/config         defaults, user file, workspace file
internal/env, proc      the process environment and the only way to start a subprocess
cmd/mkranking           maintainers only: rebuilds data/ from the sources
cmd/nocomments          a check that the code ships without comments
```

## Rules

- Dependencies point inwards. Packages define the small interfaces they consume
  (`author.Notes`, `review.Verifier`, `proc.Runner`) and `app` wires the real
  implementations in.
- No `init()`, no package-level mutable state, no `os.Getenv` outside `env`
  (everything reads `env.Env.Getenv`), no `os.Exit` outside `main`, no
  subprocess outside `proc`. `.golangci.yml` enforces most of it.
- Nothing is cached by modification time. Staleness is decided by digests: of the
  text and the sidecar for readings, of the notes, clip keys and templates for a
  build.
- Files are replaced atomically (written to a temporary file, then renamed); logs
  are only appended to.

## Data flow

1. `add` picks words from `internal/ranking` (or looks the named words up in
   `internal/lexicon`), drafts a note from the dictionary entry and sends batches
   to Mistral. Each answer passes the gates (markup, length, vowel endings,
   headword, vocabulary) and is appended to `notes.jsonl` under the lock.
2. `check` sends every Arabic word of changed notes to the sidecar, which answers
   with CAMeL's analyses and CATT's and CAMeL's readings. The raw readings are
   kept in `state/readings.jsonl`; whether a word is flagged is derived from them
   in Go each time, so a change to the comparison rules never needs a recheck.
3. `review` serves a page on `127.0.0.1` behind a random path token. Each action
   carries the revision of the note it was made on, so a note changed meanwhile
   by `add` or another tab gets a conflict instead of being overwritten.
4. `audio` computes the clips every note wants for the configured voice, skips
   those that exist and synthesises the rest.
5. `build` renders the notes, the clips that exist and the check results into a
   byte-reproducible `.apkg`.

## The Python sidecar

`python/mufradat_sidecar` speaks a line-delimited JSON protocol over stdio (version 1):
a hello line, then `check` requests answered with progress, per-item readings and
a final `done`. One warm process serves a whole `check` or `review` session.
`setup` builds the environment from a hash-pinned `requirements.lock` with a CPU
only PyTorch and installs the CAMeL data; it is shared by every workspace and
rebuilt only when the lock changes.

## The shipped data

`data/` holds the ranked list, the dictionary, the spelling index used to
recognise words in sentences, and the checksums of what they were built from.
`cmd/mkranking` regenerates all of it deterministically; see
[`data/SOURCES.md`](../data/SOURCES.md).
