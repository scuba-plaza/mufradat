# Troubleshooting

Run `mufradat doctor` first; it checks the workspace, the settings, the Mistral
key, the Google credentials (when `audio.provider` is `google`) and the vowel
checkers and says how to fix what it finds.

## "rate limited" or "the service is busy" (exit 75)

Everything `add` finished is already saved. Run the same command again a little
later; it continues with the next words that have no note. Rate limits and
server errors are retried by themselves a few times first, and `audio` keeps the
clips it made.

## "no Mistral API key" or "rejected the credentials" (exit 3)

Create an API key in the Mistral console, then either set `MISTRAL_API_KEY` or
save the key in a file (a newline at the end is fine) and run
`mufradat auth mistral --import FILE`. `mufradat auth mistral` shows which source
the key in use comes from, and `doctor` asks Mistral whether it accepts the key.

## Some clips were "blocked by Mistral's moderation"

Mistral's speech service refuses a few harmless sentences. The decision belongs
to the service, is the same every time for the same text, and cannot be
configured, so `audio` does not retry those clips; it makes all the others and
lists the blocked ones. Reword the example sentence in `review`, or switch with
`mufradat config set audio.provider google` and run `audio` again. Clips of both
providers are kept, so switching back finds the Mistral ones.

## No audio / "no Google credentials" (`audio.provider = google`)

Create a service account in a Google Cloud project with the Cloud
Text-to-Speech API enabled, download its JSON key, and run
`mufradat auth google --import key.json`. The key is copied into
`~/.config/mufradat/` with mode 0600, so the original can be deleted. If
you use `gcloud auth application-default login` credentials and Google asks for
a billing project, set it with `mufradat config set google.quota_project
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
