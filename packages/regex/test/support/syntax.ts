import type { Random } from "./random";

export type PerlLetter = "d" | "D" | "w" | "W" | "s" | "S";

export type ClassItem =
  | { kind: "literal"; code: number }
  | { kind: "range"; from: number; to: number }
  | { kind: "perl"; letter: PerlLetter };

export type Node =
  | { kind: "literal"; code: number; escaped: boolean }
  | { kind: "any" }
  | { kind: "perl"; letter: PerlLetter }
  | { kind: "class"; negated: boolean; items: ClassItem[] }
  | { kind: "start" }
  | { kind: "end" }
  | { kind: "group"; body: Expression }
  | { kind: "repeat"; body: Node; min: number; max: number | null; counted: boolean };

export type Expression = Node[][];

interface Dialect {
  literal(code: number, escaped: boolean): string;
  classLiteral(code: number): string;
  any: string;
  perl(letter: PerlLetter): string;
  classPerl(letter: PerlLetter): string;
  groupOpen: string;
}

const char = (code: number) => String.fromCodePoint(code);
const hex = (code: number) => `\\u{${code.toString(16)}}`;

const ENGINE_META = new Set([..."\\^$.|?*+()[]{}"]);
const ENGINE_CLASS_META = new Set([..."\\[]^-"]);

const engine: Dialect = {
  literal: (code, escaped) => (escaped || ENGINE_META.has(char(code)) ? `\\${char(code)}` : char(code)),
  classLiteral: (code) => (ENGINE_CLASS_META.has(char(code)) ? `\\${char(code)}` : char(code)),
  any: ".",
  perl: (letter) => `\\${letter}`,
  classPerl: (letter) => `\\${letter}`,
  groupOpen: "(",
};

const NATIVE_PERL: Record<PerlLetter, string> = {
  d: "\\p{Nd}",
  D: "\\P{Nd}",
  w: "[\\p{L}\\p{N}_]",
  W: "[^\\p{L}\\p{N}_]",
  s: "\\s",
  S: "\\S",
};

const native: Dialect = {
  literal: (code) => hex(code),
  classLiteral: (code) => hex(code),
  any: "[^]",
  perl: (letter) => NATIVE_PERL[letter],
  classPerl: (letter) => (letter === "w" ? "\\p{L}\\p{N}_" : NATIVE_PERL[letter]),
  groupOpen: "(?:",
};

function quantifier(node: { min: number; max: number | null; counted: boolean }): string {
  if (node.counted) return `{${node.min},${node.max ?? ""}}`;
  if (node.max === 1) return "?";
  return node.min === 0 ? "*" : "+";
}

function renderItem(item: ClassItem, dialect: Dialect): string {
  if (item.kind === "literal") return dialect.classLiteral(item.code);
  if (item.kind === "range") return `${dialect.classLiteral(item.from)}-${dialect.classLiteral(item.to)}`;
  return dialect.classPerl(item.letter);
}

function renderNode(node: Node, dialect: Dialect): string {
  switch (node.kind) {
    case "literal":
      return dialect.literal(node.code, node.escaped);
    case "any":
      return dialect.any;
    case "perl":
      return dialect.perl(node.letter);
    case "class":
      return `[${node.negated ? "^" : ""}${node.items.map((item) => renderItem(item, dialect)).join("")}]`;
    case "start":
      return "^";
    case "end":
      return "$";
    case "group":
      return `${dialect.groupOpen}${renderExpression(node.body, dialect)})`;
    case "repeat":
      return renderNode(node.body, dialect) + quantifier(node);
  }
}

const renderExpression = (expression: Expression, dialect: Dialect): string =>
  expression.map((branch) => branch.map((node) => renderNode(node, dialect)).join("")).join("|");

export const renderEngine = (expression: Expression): string => renderExpression(expression, engine);

export const renderNative = (expression: Expression): string => renderExpression(expression, native);

const code = (text: string): number => text.codePointAt(0)!;

const LETTERS = [..."aaabbbccd"].map(code);
const OTHERS = [..."1_ ٥كتب😀𝔸é"].map(code);
const PUNCTUATION = [..."*.+?()[]{}|^$\\-/!,"].map(code);
const RANGES = [
  ["a", "c"],
  ["a", "z"],
  ["b", "d"],
  ["0", "9"],
  ["\u0627", "\u064a"],
  ["ك", "ي"],
  ["\u{1F600}", "\u{1F64F}"],
  [" ", "/"],
].map(([from, to]): readonly [number, number] => [code(from), code(to)]);
const PERL_LETTERS: readonly PerlLetter[] = ["d", "D", "w", "W", "s", "S"];
const SIMPLE = { "?": { min: 0, max: 1 }, "*": { min: 0, max: null }, "+": { min: 1, max: null } };

const NOISE = [..."abc1٥ك_ -\n😀𝔸é"];

const MAX_DEPTH = 3;

const literalCode = (random: Random): number =>
  random.pick(random.weighted([[10, LETTERS], [4, OTHERS], [2, PUNCTUATION]]));

function classItem(random: Random): ClassItem {
  if (random.chance(0.35)) {
    const [from, to] = random.pick(RANGES);
    return { kind: "range", from, to };
  }
  if (random.chance(0.3)) return { kind: "perl", letter: random.pick(PERL_LETTERS) };
  return { kind: "literal", code: literalCode(random) };
}

function characterClass(random: Random): Node {
  const items: ClassItem[] = [];
  for (let k = random.between(1, 4); k > 0; k--) items.push(classItem(random));
  return { kind: "class", negated: random.chance(0.3), items };
}

function atom(random: Random, depth: number): Node {
  const choice = random.weighted<"literal" | "any" | "perl" | "class" | "start" | "end" | "group">([
    [40, "literal"],
    [6, "any"],
    [8, "perl"],
    [12, "class"],
    [5, "start"],
    [5, "end"],
    [depth < MAX_DEPTH ? 12 : 0, "group"],
  ]);
  switch (choice) {
    case "literal": {
      const value = literalCode(random);
      return { kind: "literal", code: value, escaped: PUNCTUATION.includes(value) && random.chance(0.3) };
    }
    case "any":
      return { kind: "any" };
    case "perl":
      return { kind: "perl", letter: random.pick(PERL_LETTERS) };
    case "class":
      return characterClass(random);
    case "start":
      return { kind: "start" };
    case "end":
      return { kind: "end" };
    case "group":
      return { kind: "group", body: expression(random, depth + 1) };
  }
}

function bounds(random: Random, large: boolean): { min: number; max: number | null } {
  const top = large && random.chance(0.15) ? 100 : random.chance(0.8) ? 3 : 8;
  const min = random.between(0, top);
  return { min, max: random.chance(0.2) ? null : random.between(min, Math.min(top, min + 6)) };
}

function piece(random: Random, depth: number): Node {
  const body = atom(random, depth);
  if (body.kind === "start" || body.kind === "end") return body;
  const form = random.weighted<"none" | "?" | "*" | "+" | "counted">([
    [55, "none"],
    [8, "?"],
    [8, "*"],
    [8, "+"],
    [21, "counted"],
  ]);
  if (form === "none") return body;
  if (form === "counted") return { kind: "repeat", body, counted: true, ...bounds(random, depth === 0 && body.kind !== "group") };
  return { kind: "repeat", body, counted: false, ...SIMPLE[form] };
}

function expression(random: Random, depth: number): Expression {
  const branches: Expression = [];
  for (let b = random.weighted([[60, 1], [30, 2], [10, 3]]); b > 0; b--) {
    const sequence: Node[] = [];
    for (let n = random.weighted([[10, 0], [30, 1], [30, 2], [20, 3], [10, 4]]); n > 0; n--) sequence.push(piece(random, depth));
    branches.push(sequence);
  }
  return branches;
}

export function generate(random: Random, maxLength: number): Expression {
  for (;;) {
    const candidate = expression(random, 0);
    if (renderEngine(candidate).length <= maxLength) return candidate;
  }
}

function charsOf(expr: Expression, found: Set<string>): void {
  for (const branch of expr) {
    for (const node of branch) {
      const target = node.kind === "repeat" ? node.body : node;
      if (target.kind === "literal") found.add(char(target.code));
      else if (target.kind === "class") {
        for (const item of target.items) {
          if (item.kind === "literal") found.add(char(item.code));
          else if (item.kind === "range") [item.from, item.to].forEach((end) => found.add(char(end)));
        }
      } else if (target.kind === "group") charsOf(target.body, found);
    }
  }
}

export function textsFor(random: Random, expr: Expression, count: number): string[] {
  const own = new Set<string>();
  charsOf(expr, own);
  const pool = own.size === 0 ? NOISE : [...own];
  const texts = [""];
  while (texts.length < count) {
    const length = random.chance(0.85) ? random.between(0, 7) : random.between(8, 24);
    let text = "";
    for (let k = 0; k < length; k++) text += random.chance(0.75) ? random.pick(pool) : random.pick(NOISE);
    texts.push(text);
  }
  return texts;
}
