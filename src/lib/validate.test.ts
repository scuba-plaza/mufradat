import { describe, expect, it } from "vitest";
import { autoMark, markSelection, skeleton } from "./validate";

describe("marking the word of the card in an unmarked example", () => {
  it("marks the headword as it stands in the sentence, whatever its vowels", () => {
    expect(autoMark("لَدَيْهِ كِتَابٌ جَمِيلٌ", ["كِتَاب"])).toBe("لَدَيْهِ [كِتَابٌ] جَمِيلٌ");
  });

  it("marks a word that has a preposition or conjunction attached, leaving that out", () => {
    expect(autoMark("قَرَأْتُ بِالْكِتَابِ", ["كِتَاب"])).toBe("قَرَأْتُ بِ[الْكِتَابِ]");
  });

  it("marks a word that has a pronoun attached, leaving the pronoun out", () => {
    expect(autoMark("كِتَابُهُ جَمِيلٌ", ["كِتَاب"])).toBe("[كِتَابُ]هُ جَمِيلٌ");
  });

  it("tries the forms in turn when the headword is not in the sentence", () => {
    expect(autoMark("عِنْدِي كُتُبٌ كَثِيرَةٌ", ["كِتَاب", "كُتُب"])).toBe("عِنْدِي [كُتُبٌ] كَثِيرَةٌ");
  });

  it("does not look at later words once the first stands twice", () => {
    expect(autoMark("الْكِتَابُ فَوْقَ الْكِتَابِ وَكُتُبٌ", ["كِتَاب", "كُتُب"])).toBeNull();
  });

  it("keeps Arabic-Indic digits when it takes the vowel marks away", () => {
    expect(skeleton("كِتَابٌ ٣")).toBe("كتاب ٣");
  });

  it("copes with characters in a headword that mean something in a pattern", () => {
    expect(autoMark("لَدَيْهِ كِتَابٌ جَمِيلٌ", ["[كتاب", "(كتاب+", "كِتَاب"])).toBe("لَدَيْهِ [كِتَابٌ] جَمِيلٌ");
  });

  it("does not guess when the word is absent or stands twice", () => {
    expect(autoMark("ذَهَبَ الْوَلَدُ إِلَى الْمَدْرَسَةِ", ["كِتَاب"])).toBeNull();
    expect(autoMark("الْكِتَابُ فَوْقَ الْكِتَابِ", ["كِتَاب"])).toBeNull();
  });
});

describe("marking the letters that are selected", () => {
  const select = (draft: string, text: string) => [draft.indexOf(text), draft.indexOf(text) + text.length] as const;
  const marked = (draft: string, text: string) => markSelection(draft, ...select(draft, text));

  it("puts the brackets around exactly the selected letters, part of a word included", () => {
    expect(marked("لَدَيْهِ كِتَابٌ جَمِيلٌ", "كِتَاب")).toBe("لَدَيْهِ [كِتَاب]ٌ جَمِيلٌ");
  });

  it("moves the mark when other letters are selected", () => {
    expect(marked("لَدَيْهِ [كِتَابٌ] جَمِيلٌ", "جَمِيلٌ")).toBe("لَدَيْهِ كِتَابٌ [جَمِيلٌ]");
  });

  it("takes the mark away when its letters are selected again", () => {
    expect(marked("لَدَيْهِ [كِتَابٌ] جَمِيلٌ", "كِتَابٌ")).toBe("لَدَيْهِ كِتَابٌ جَمِيلٌ");
  });

  it("leaves out the spaces around the selection", () => {
    expect(markSelection("لَدَيْهِ كِتَابٌ جَمِيلٌ", 8, 17)).toBe("لَدَيْهِ [كِتَابٌ] جَمِيلٌ");
  });

  it("changes nothing for an empty selection or for the whole sentence", () => {
    const draft = "لَدَيْهِ كِتَابٌ جَمِيلٌ";
    expect(markSelection(draft, 4, 4)).toBe(draft);
    expect(markSelection(draft, 0, draft.length)).toBe(draft);
  });

  it("counts a lone bracket in the sentence as no mark", () => {
    const draft = "لَدَيْهِ [كِتَابٌ جَمِيلٌ";
    expect(markSelection(draft, draft.indexOf("جَمِيلٌ"), draft.length)).toBe("لَدَيْهِ كِتَابٌ [جَمِيلٌ]");
  });
});
