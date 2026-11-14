import { describe, expect, it } from "vitest";
import { parse } from "../src/syntax/parse";
import type { Node } from "../src/syntax/types";

const lit = (char: string): Node => ({ kind: "literal", code: char.codePointAt(0)! });
const concat = (...items: Node[]): Node => ({ kind: "concat", items });
const alternate = (...branches: Node[]): Node => ({ kind: "alternate", branches });

function tree(pattern: string): Node {
  const parsed = parse(pattern);
  if ("error" in parsed) throw new Error(`${pattern} was refused with ${parsed.error.code}`);
  return parsed.tree;
}

describe("the syntax tree of literals, alternation and groups", () => {
  it("is one literal for one character and a concatenation for several", () => {
    expect(tree("a")).toEqual(lit("a"));
    expect(tree("ab")).toEqual(concat(lit("a"), lit("b")));
    expect(tree("a\u{1F600}")).toEqual(concat(lit("a"), lit("\u{1F600}")));
  });

  it("binds a concatenation tighter than the pipe", () => {
    expect(tree("ab|cd")).toEqual(alternate(concat(lit("a"), lit("b")), concat(lit("c"), lit("d"))));
    expect(tree("a|b|c")).toEqual(alternate(lit("a"), lit("b"), lit("c")));
  });

  it("is an empty concatenation for the empty pattern, an empty group and an empty branch", () => {
    expect(tree("")).toEqual(concat());
    expect(tree("()")).toEqual(concat());
    expect(tree("a|")).toEqual(alternate(lit("a"), concat()));
  });

  it("leaves no node for a group, only the grouping it stands for", () => {
    expect(tree("(a)")).toEqual(lit("a"));
    expect(tree("(a|b)c")).toEqual(concat(alternate(lit("a"), lit("b")), lit("c")));
    expect(tree("a(b|c)")).toEqual(concat(lit("a"), alternate(lit("b"), lit("c"))));
  });
});

describe("the syntax tree of the dot and character classes", () => {
  it("takes the dot as a class that holds everything but the newline", () => {
    expect(tree(".")).toEqual({ kind: "class", negated: true, items: [{ kind: "range", from: 10, to: 10 }] });
  });

  it("keeps a class as its negation flag and its members, a single character being a one-character range", () => {
    expect(tree("[^a-c_]")).toEqual({
      kind: "class",
      negated: true,
      items: [
        { kind: "range", from: 97, to: 99 },
        { kind: "range", from: 95, to: 95 },
      ],
    });
  });
});

