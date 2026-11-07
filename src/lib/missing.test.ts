import { describe, expect, it } from "vitest";
import { missingKinds, missingParts } from "./missing";
import type { Clip, Note } from "./types";

const clip = (over: Partial<Clip> = {}): Clip => ({ key: "abc", source: "tts", text: "", error: null, outdated: false, ...over });

const bare: Note = {
  id: "1", lemma: "كِتَاب", english: "book", pos: "noun", gender: "", root: "", verbForm: "", hint: "",
  example: "", exampleEn: "", exampleError: null, rank: 120, status: "draft", lemmaSource: "wiktionary",
  exampleSource: "", createdAt: "", updatedAt: "", forms: [], audio: {}, tags: [],
};

describe("what a card still lacks", () => {
  it("names the example and the audio of the word on a bare card", () => {
    expect(missingParts(bare)).toEqual(["an example", "audio for the word"]);
  });

  it("asks for the translation once there is an example, and for the example's audio", () => {
    const n = { ...bare, example: "هٰذَا [كِتَابٌ]", audio: { word: clip() } };
    expect(missingParts(n)).toEqual(["a translation of the example", "audio for the example"]);
  });

  it("asks for the target word to be marked when the example has no marked word", () => {
    const n = { ...bare, example: "هٰذَا كِتَابٌ", exampleEn: "This is a book.", audio: { word: clip(), example: clip() } };
    expect(missingParts(n)).toEqual(["the target word marked in the example"]);
  });

  it("names each form that has no audio by its label", () => {
    const n = { ...bare, forms: [{ id: "f1", label: "pl.", arabic: "كُتُب" }], audio: { word: clip() } };
    expect(missingParts(n)).toEqual(["an example", "audio for pl."]);
  });

  it("asks again for a clip whose text changed since it was made", () => {
    const n = { ...bare, audio: { word: clip({ outdated: true }) } };
    expect(missingParts(n)).toEqual(["an example", "new audio for the word"]);
  });

  it("is empty for a card that has everything", () => {
    const n = {
      ...bare, example: "هٰذَا [كِتَابٌ]", exampleEn: "This is a book.",
      forms: [{ id: "f1", label: "pl.", arabic: "كُتُب" }],
      audio: { word: clip(), f1: clip(), example: clip() },
    };
    expect(missingParts(n)).toEqual([]);
  });
});

describe("what a card lacks, by kind", () => {
  it("lists each kind once, in the order of the card", () => {
    const n = { ...bare, forms: [{ id: "f1", label: "pl.", arabic: "كُتُب" }] };
    expect(missingKinds(n)).toEqual(["example", "audio"]);
  });

  it("names the translation and the target of a written example, and audio that is outdated", () => {
    const n = { ...bare, example: "هٰذَا كِتَابٌ", audio: { word: clip({ outdated: true }), example: clip() } };
    expect(missingKinds(n)).toEqual(["translation", "target", "audio"]);
  });

  it("is empty for a card that has everything", () => {
    const n = { ...bare, example: "هٰذَا [كِتَابٌ]", exampleEn: "This is a book.", audio: { word: clip(), example: clip() } };
    expect(missingKinds(n)).toEqual([]);
  });
});
