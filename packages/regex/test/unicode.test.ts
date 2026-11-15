import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { generate, matchRanges } from "../scripts/generate-unicode";
import { parse } from "../src/syntax/parse";
import { contains, negate } from "../src/program/ranges";
import { PROPERTIES, SPACE, UNICODE_VERSION } from "../src/unicode/tables";

const TABLES_FILE = new URL("../src/unicode/tables.ts", import.meta.url);

const CATEGORIES = [
  "C", "Cc", "Cf", "Co", "Cs", "L", "Ll", "Lm", "Lo", "Lt", "Lu", "M", "Mc", "Me", "Mn", "N", "Nd", "Nl", "No",
  "P", "Pc", "Pd", "Pe", "Pf", "Pi", "Po", "Ps", "S", "Sc", "Sk", "Sm", "So", "Z", "Zl", "Zp", "Zs",
];

describe("the unicode tables", () => {
  it("are for the Unicode version of the running V8", () => {
    expect(UNICODE_VERSION).toBe(process.versions.unicode);
  });

  it("reproduce byte for byte from the script", () => {
    expect(generate()).toBe(readFileSync(TABLES_FILE, "utf8"));
  });

  it("hold, for every name, exactly the code points V8 does not give \\P{name}", () => {
    const wrong: string[] = [];
    for (const [name, ranges] of Object.entries(PROPERTIES)) {
      const property = CATEGORIES.includes(name) ? name : `Script=${name}`;
      if (JSON.stringify(negate(ranges)) !== JSON.stringify(matchRanges(`\\P{${property}}`))) wrong.push(name);
    }
    expect(wrong).toEqual([]);
  });

  it("name the general categories and the scripts in RE2 and Go spelling, and nothing else", () => {
    const names = Object.keys(PROPERTIES);
    for (const category of CATEGORIES) expect(names).toContain(category);
    for (const script of ["Arabic", "Latin", "Greek", "Han", "Common", "Inherited", "Old_Italic", "Nko", "SignWriting", "Phags_Pa"]) {
      expect(names).toContain(script);
    }
    for (const absent of ["Cn", "LC", "Any", "Letter", "Uppercase_Letter", "Unknown", "Katakana_Or_Hiragana", "lu", "arabic", "Script=Arabic"]) {
      expect(names).not.toContain(absent);
    }
    expect(names.length).toBe(CATEGORIES.length + 174);
  });

  it("put well known characters in the sets they belong to", () => {
    expect(contains(PROPERTIES.Nd, 0x665)).toBe(true);
    expect(contains(PROPERTIES.Nd, 0xb2)).toBe(false);
    expect(contains(PROPERTIES.N, 0xb2)).toBe(true);
    expect(contains(PROPERTIES.L, 0x643)).toBe(true);
    expect(contains(PROPERTIES.Arabic, 0x643)).toBe(true);
    expect(contains(PROPERTIES.Latin, 0x61)).toBe(true);
    expect(contains(PROPERTIES.Lu, 0x1d538)).toBe(true);
    expect(contains(PROPERTIES.Cs, 0xd800)).toBe(true);
    expect(contains(PROPERTIES.Cs, 0xdfff)).toBe(true);
    expect(contains(PROPERTIES.Cs, 0xe000)).toBe(false);
    expect(contains(PROPERTIES.Co, 0xe000)).toBe(true);
    expect(contains(PROPERTIES.Co, 0x10fffd)).toBe(true);
    expect(contains(PROPERTIES.Co, 0x10ffff)).toBe(false);
    expect(contains(PROPERTIES.C, 0x10ffff)).toBe(true);
  });

  it("give \\s exactly the white space and line terminators of ECMAScript", () => {
    expect([...SPACE]).toEqual([9, 13, 32, 32, 160, 160, 5760, 5760, 8192, 8202, 8232, 8233, 8239, 8239, 8287, 8287, 12288, 12288, 65279, 65279]);
  });

  it("are all accepted by the parser, as \\p{name} and \\P{name}", () => {
    const refused: string[] = [];
    for (const name of Object.keys(PROPERTIES)) {
      for (const letter of ["p", "P"]) if ("error" in parse(`\\${letter}{${name}}`)) refused.push(`\\${letter}{${name}}`);
    }
    expect(refused).toEqual([]);
  });
});
