#!/bin/sh
set -eu
rm -rf completions
mkdir completions
go run ./cmd/mufradat completion bash > completions/mufradat.bash
go run ./cmd/mufradat completion zsh > completions/_mufradat
go run ./cmd/mufradat completion fish > completions/mufradat.fish
