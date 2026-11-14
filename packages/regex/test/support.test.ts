import { describe, expect, it } from "vitest";
import { renderEngine, renderNative, type Expression } from "./support/generator";
import { nativeMatcher } from "./support/native";
import { TIMEOUT } from "./support/watchdog";

describe("the native RegExp runner", () => {
  it("gives up on a native pattern that backtracks without end, and says so", () => {
    const stuck = nativeMatcher("^(?:\\u{61}+)+$");
    expect(stuck(["a".repeat(40) + "!", "aaa", "b"])).toEqual([TIMEOUT, true, false]);
  });

  it("starts a match only at a code point, never inside a surrogate pair", () => {
    const word = "[\\p{L}\\p{N}_]";
    const notBoundary = nativeMatcher(`(?:(?<=${word})(?=${word})|(?<!${word})(?!${word}))`);
    expect(notBoundary(["\u{1D538}", "b\u{1F600}\u{665}", "ab", ""])).toEqual([false, false, true, true]);
  });
});

describe("the generator's two renderings of one expression", () => {
  it("writes an engine pattern and native source that stand for the same syntax", () => {
    const plain = (code: number) => ({ code, form: "plain" as const });
    const expression: Expression = [
      [
        { kind: "start" },
        { kind: "repeat", body: { kind: "literal", char: plain(0x2a) }, min: 2, max: null, form: "range" },
        {
          kind: "class",
          negated: true,
          items: [{ kind: "range", from: plain(0x61), to: plain(0x63) }, { kind: "perl", letter: "W" }, { kind: "literal", char: plain(0x2d) }],
        },
        { kind: "group", body: [[{ kind: "any" }, { kind: "perl", letter: "w" }], [{ kind: "literal", char: { code: 0x7c, form: "backslash" } }]] },
        { kind: "boundary", negated: false },
        { kind: "property", name: "Arabic", negated: false },
        { kind: "repeat", body: { kind: "literal", char: { code: 0x41, form: "hexBraced" } }, min: 3, max: 3, form: "exact" },
        { kind: "literal", char: { code: 0x0a, form: "control" } },
        { kind: "literal", char: { code: 0x4a, form: "hex2" } },
        { kind: "literal", char: { code: 0x643, form: "unicode4" } },
        { kind: "end" },
      ],
    ];
    expect(renderEngine(expression)).toBe("^\\*{2,}[^a-c\\W\\-](.\\w|\\|)\\b\\p{Arabic}\\x{41}{3}\\n\\x4a\\u0643$");
    expect(renderNative(expression)).toBe(
      "(?:^)\\u{2a}{2,}[^\\u{61}-\\u{63}[^\\p{L}\\p{N}_]\\u{2d}](?:[^\\n][\\p{L}\\p{N}_]|\\u{7c})" +
        "(?:(?<=[\\p{L}\\p{N}_])(?![\\p{L}\\p{N}_])|(?<![\\p{L}\\p{N}_])(?=[\\p{L}\\p{N}_]))" +
        "\\p{Script=Arabic}\\u{41}{3}\\u{a}\\u{4a}\\u{643}(?:$)",
    );
  });
});
