import type { MissingKind, Note, WordStatus } from "./types";
import { splitExample } from "./validate";

/** Where a card stands: approved, ready when it lacks nothing and only approving is left, otherwise still in progress. */
export function wordStatus(n: Note): Exclude<WordStatus, "todo"> {
  if (n.status === "approved") return "approved";
  return gaps(n).length === 0 ? "ready" : "progress";
}

function gaps(n: Note): { kind: MissingKind; text: string }[] {
  const missing: { kind: MissingKind; text: string }[] = [];
  if (n.example.trim() === "") missing.push({ kind: "example", text: "an example" });
  else {
    if (n.exampleEn.trim() === "") missing.push({ kind: "translation", text: "a translation of the example" });
    if (!splitExample(n.example)) missing.push({ kind: "target", text: "the target word marked in the example" });
  }

  const clips = [
    { slot: "word", name: "the word" },
    ...n.forms.map((f) => ({ slot: f.id, name: f.label })),
    ...(n.example.trim() === "" ? [] : [{ slot: "example", name: "the example" }]),
  ];
  for (const { slot, name } of clips) {
    const clip = n.audio[slot];
    if (!clip?.key) missing.push({ kind: "audio", text: `audio for ${name}` });
    else if (clip.outdated) missing.push({ kind: "audio", text: `new audio for ${name}` });
  }
  return missing;
}

/** What a card still lacks, in words for the person making it; empty once it has all of it. */
export const missingParts = (n: Note): string[] => gaps(n).map((g) => g.text);

export const missingKinds = (n: Note): MissingKind[] => [...new Set(gaps(n).map((g) => g.kind))];
