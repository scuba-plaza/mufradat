# Troubleshooting

Run `mufradat doctor` first; it checks the workspace, the settings, Claude
Code, the Google credentials and the vowel checkers and says how to fix what it
finds.

## "Your Claude plan's usage limit was reached" (exit 75)

Everything `add` finished is already saved. Run the same command again after the
time it names; it continues with the next words that have no note. Rate limits
and a busy service are retried by themselves a few times first.

## "Install Claude Code" or "sign in"

`add` runs `claude -p` with your own login. Install Claude Code, run `claude`
once to sign in, then try again. `doctor` shows the version it found.

## No audio / "no Google credentials"

Create a service account in a Google Cloud project with the Cloud
Text-to-Speech API enabled, download its JSON key, and run
`mufradat auth google --import key.json`. The key is copied into
`~/.config/mufradat/` with mode 0600, so the original can be deleted. If
you use `gcloud auth application-default login` credentials and Google asks for
a billing project, set it with `mufradat config set tts.quota_project
PROJECT --user`.

Use `mufradat audio --dry-run` to see how many characters would be sent
before spending anything.

## `setup` or `check` fails on NixOS: `libstdc++.so.6: cannot open shared object file`

NixOS keeps C++ libraries out of the default search path, so the Python wheels
cannot load them. The Nix package of mufradat sets `LD_LIBRARY_PATH` for you:

```sh
nix run github:scuba-plaza/mufradat -- setup
```

If you installed it another way, point `LD_LIBRARY_PATH` at the directories that
hold `libstdc++.so.6` and `libz.so.1` before running `setup` and `check`.
`doctor` recognises this failure and says so.

## `setup` is slow or runs out of disk

The environment is about 1.4 GB and the CAMeL language data about 550 MB, both
under `~/.local/share/mufradat/`. The first test run also downloads CATT's
model weights (a few hundred MB) from its GitHub release into the environment.
`setup` is run once; a second run only repeats the test. `setup --force` rebuilds.

`check` is optional: without it every card is tagged `check::unverified` and
the rest of the tool works.

## `check` says everything is checked but the review page is empty

`review` lists only flagged notes you have not dealt with. `review --all` lists
every note, flagged or not.

## Two commands at once

`add` and `review` can run side by side. Notes that `add` writes while a review is
open are not in that review; run `review` again to see them. Two writers never
overwrite each other because each rewrite happens under the workspace lock. On a network
file system the lock is not reliable: keep the workspace on a local disk.

## The `.apkg` is different each time

It should not be: building twice from the same inputs gives the same bytes. Each
note carries the time it was last changed; set `SOURCE_DATE_EPOCH` to give every
note the same fixed time instead.

## Where is everything?

`mufradat config path` prints the user config file, the data and cache
directories, and the workspace in use and why it was chosen.
