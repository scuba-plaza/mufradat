# The workspace

Everything that belongs to you lives in one directory, the workspace. The tool
itself never writes next to its own files, so it can be installed read-only
(a Nix store path, `/usr/local/bin`) and shared by several workspaces.

```
mufradat.toml      settings and the marker that makes the directory a workspace
notes.jsonl            one note per line: the source of truth
decisions.jsonl        what you decided in review (accepted readings, dismissed flags)
audio/ab/<key>.mp3     the clips; paid for, so never refreshed by themselves
state/                 can be deleted: readings.jsonl, usage.jsonl, last-build.json, lock, voice-samples/
out/                   the built .apkg
```

`mufradat init --git` also writes a `.gitignore` for `state/` and `out/`, so
the notes, decisions, audio and settings can live in a git repository.

## Finding the workspace

In this order:

1. `-C DIR` / `--workspace DIR`
2. `MUFRADAT_WORKSPACE`
3. the nearest directory above the current one that holds `mufradat.toml`
4. `workspace =` in `~/.config/mufradat/config.toml`
5. `~/.local/share/mufradat/default`

`mufradat config path` prints which one was used and where everything else is.

## What may be edited by hand

`notes.jsonl` is plain JSON lines and safe to edit or `git diff` while no other
`mufradat` command is running; a command that finds a note it cannot read
names the line. Keep the shape of a note:

```json
{"id":"n_7k3q9x2m4dpa","seq":37,"rank":412,"origin":"ranked","lemma":"كِتَاب","pos":"noun","gender":"m","root":"ك ت ب","forms":[{"label":"pl.","ar":"كُتُب"}],"english":"book","example":"قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ.","example_en":"I read the book at home.","created":"2026-10-08T12:00:00Z","updated":"2026-10-08T12:00:00Z"}
```

- `id` never changes; it is also the Anki note GUID, so renaming a headword does
  not create a second card.
- `rank` is the word's place in the shipped list, absent for words you added by
  name that the list does not hold. `seq` is the order the notes were created in.
  Study order is rank first, then unranked words by `seq`.
- `example` marks the word being taught with one pair of square brackets. The
  deck shows it in bold; everything else is escaped.
- A bare case ending (a final letter with no vowel mark) makes a note invalid.
  `add` and `review` refuse it; `status` counts invalid notes you wrote by hand
  and points you to `review --all` to fix them.

Writes take a lock on `state/lock` only while a file is being rewritten, never
across a Claude or Google call, so `review` and `add` can run at the same time
without losing each other's notes. The lock is a `flock`, which does not work
reliably on network file systems such as NFS; keep the workspace on a local disk.

## Audio

A clip's name is derived from the voice, the speaking rate and the exact spoken
text. Change the voice and new clips are made for it; change it back and the old
ones are found again. `mufradat audio gc` lists clips no note needs with the
current voice and `--delete` removes them. `audio --dry-run` says how many
characters would be sent to Google before anything is.

## Deleting things

- `state/` is safe to delete; `check` will read the notes again.
- `out/` is rebuilt by `build`.
- Never delete `audio/` casually: every clip costs money to make.
