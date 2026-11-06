import { describe, expect, it } from "vitest";
import { closeness, fold } from "./fuzzy";

describe("folding text so that spellings compare equal", () => {
  it("drops the vowel marks and the tatweel of Arabic", () => {
    expect(fold("كِتَابٌ")).toBe("كتاب");
    expect(fold("كـتاب")).toBe("كتاب");
  });

  it("folds the hamza and alef forms into one alef", () => {
    expect(fold("أَنَّ")).toBe("ان");
    expect(fold("إِنْ")).toBe("ان");
    expect(fold("آنَ")).toBe("ان");
  });

  it("folds alef maqsura into ya and ta marbuta into ha", () => {
    expect(fold("عَلَى")).toBe("علي");
    expect(fold("مَدْرَسَة")).toBe("مدرسه");
  });

  it("lowercases Latin text", () => {
    expect(fold("Book")).toBe("book");
  });

  it("keeps Arabic-Indic digits and leaves nothing of vowel marks alone", () => {
    expect(fold("٢٩")).toBe("٢٩");
    expect(fold("َـ")).toBe("");
  });
});

describe("how close a typed word is to a text", () => {
  it("is 0 for the whole text, 1 for one of its words, 2 for the start of a word, 3 for part of a word", () => {
    expect(closeness("book", "book")).toBe(0);
    expect(closeness("book", "the book")).toBe(1);
    expect(closeness("boo", "the book")).toBe(2);
    expect(closeness("ook", "the book")).toBe(3);
  });

  it("ignores case and, in Arabic, vowel marks and hamza, ya and ha variants", () => {
    expect(closeness("BOOK", "book")).toBe(0);
    expect(closeness("كتاب", "كِتَاب")).toBe(0);
    expect(closeness("ان", "إِنَّ")).toBe(0);
    expect(closeness("مدرسة", "مَدْرَسَه")).toBe(0);
  });

  it("is worse by the number of typos, within one for a word of 4 to 6 letters and two from 7", () => {
    expect(closeness("boook", "book")).toBe(5);
    expect(closeness("writting", "writing")).toBe(5);
    expect(closeness("wrtting", "writing")).toBe(5);
    expect(closeness("wrritting", "writing")).toBe(6);
    expect(closeness("kitaab", "kitab")).toBe(5);
    expect(closeness("kitaaab", "kitab")).toBe(6);
    expect(closeness("kitaaaab", "kitab")).toBe(Infinity);
  });

  it("counts two letters swapped as one typo", () => {
    expect(closeness("hotsel", "hostel")).toBe(5);
  });

  it("allows no typo in a word of three letters or fewer", () => {
    expect(closeness("cat", "car")).toBe(Infinity);
    expect(closeness("كتب", "كتاب")).toBe(Infinity);
  });

  it("finds nothing for a word that is not there", () => {
    expect(closeness("xyz", "book")).toBe(Infinity);
  });

  it("finds nothing for a term that is only vowel marks, and for digits that are not in the text", () => {
    expect(closeness("َ", "كتاب")).toBe(Infinity);
    expect(closeness("ـ", "كتاب")).toBe(Infinity);
    expect(closeness("٣", "كتاب")).toBe(Infinity);
    expect(closeness("٣", "الباب ٣")).toBe(1);
  });

  it("takes a phrase only as it stands in the text, without typos", () => {
    expect(closeness("to write", "to write well", true)).toBe(3);
    expect(closeness("to wrote", "to write well", true)).toBe(Infinity);
  });
});
