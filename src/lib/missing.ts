import type { Note, WordStatus } from "./types";
import { splitExample } from "./validate";

/** Where a card stands: approved, ready when it lacks nothing and only approving is left, otherwise still in progress. */
export function wordStatus(n: Note): Exclude<WordStatus, "todo"> {
  if (n.status === "approved") return "approved";
  return missingParts(n).length === 0 ? "ready" : "progress";
}

/** What a card still lacks, in words for the person making it; empty once it has all of it. */
export function missingParts(n: Note): string[] {
  const missing: string[] = [];
  if (n.example.trim() === "") missing.push("an example");
  else {
    if (n.exampleEn.trim() === "") missing.push("a translation of the example");
    if (!splitExample(n.example)) missing.push("the target word marked in the example");
  }

  const clips = [
    { slot: "word", name: "the word" },
    ...n.forms.map((f) => ({ slot: f.id, name: f.label })),
    ...(n.example.trim() === "" ? [] : [{ slot: "example", name: "the example" }]),
  ];
  for (const { slot, name } of clips) {
    const clip = n.audio[slot];
    if (!clip?.key) missing.push(`audio for ${name}`);
    else if (clip.outdated) missing.push(`new audio for ${name}`);
  }
  return missing;
}
