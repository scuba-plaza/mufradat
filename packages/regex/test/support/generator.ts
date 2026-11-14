import type { Random } from "./random";

export type Scope = "unchanged" | "all";
export type PerlLetter = "d" | "D" | "w" | "W" | "s" | "S";
export type Form = "plain" | "backslash" | "control" | "hex2" | "hexBraced" | "unicode4";

export interface Char {
  code: number;
  form: Form;
}

export type ClassItem =
  | { kind: "literal"; char: Char }
  | { kind: "range"; from: Char; to: Char }
  | { kind: "perl"; letter: PerlLetter }
  | { kind: "property"; name: string; negated: boolean };

export type Node =
  | { kind: "literal"; char: Char }
  | { kind: "any" }
  | { kind: "perl"; letter: PerlLetter }
  | { kind: "property"; name: string; negated: boolean }
  | { kind: "class"; negated: boolean; items: ClassItem[] }
  | { kind: "start" }
  | { kind: "end" }
  | { kind: "boundary"; negated: boolean }
  | { kind: "group"; body: Expression }
  | { kind: "repeat"; body: Node; min: number; max: number | null; form: "?" | "*" | "+" | "range" | "exact" };

export type Expression = Node[][];

interface Dialect {
  char(char: Char, inClass: boolean): string;
  any: string;
  perl(letter: PerlLetter, inClass: boolean): string;
  property(name: string, negated: boolean): string;
  boundary(negated: boolean): string;
  anchor(kind: "^" | "$"): string;
  groupOpen: string;
}

const text = (code: number) => String.fromCodePoint(code);
const hex = (code: number) => code.toString(16);
const pad = (code: number, width: number) => hex(code).padStart(width, "0");

const ENGINE_META = new Set([..."\\^$.|?*+()[]{}"]);
const ENGINE_CLASS_META = new Set([..."\\[]^-"]);
const CONTROL: Record<number, string> = { 10: "n", 9: "t", 13: "r", 12: "f", 11: "v" };

function engineChar({ code, form }: Char, inClass: boolean): string {
  switch (form) {
    case "plain":
      return (inClass ? ENGINE_CLASS_META : ENGINE_META).has(text(code)) ? `\\${text(code)}` : text(code);
    case "backslash":
      return `\\${text(code)}`;
    case "control":
      return `\\${CONTROL[code]}`;
    case "hex2":
      return `\\x${pad(code, 2)}`;
    case "hexBraced":
      return `\\x{${hex(code)}}`;
    case "unicode4":
      return `\\u${pad(code, 4)}`;
  }
}

const engine: Dialect = {
  char: engineChar,
  any: ".",
  perl: (letter) => `\\${letter}`,
  property: (name, negated) => `\\${negated ? "P" : "p"}{${name}}`,
  boundary: (negated) => (negated ? "\\B" : "\\b"),
  anchor: (kind) => kind,
  groupOpen: "(",
};

const NATIVE_WORD = "\\p{L}\\p{N}_";
const NATIVE_PERL: Record<PerlLetter, string> = {
  d: "\\p{Nd}",
  D: "\\P{Nd}",
  w: `[${NATIVE_WORD}]`,
  W: `[^${NATIVE_WORD}]`,
  s: "\\s",
  S: "\\S",
};

const nativeProperty = (name: string, negated: boolean) =>
  `\\${negated ? "P" : "p"}{${/^[A-Z][a-z]?$/.test(name) ? "" : "Script="}${name}}`;

const native: Dialect = {
  char: ({ code }) => `\\u{${hex(code)}}`,
  any: "[^\\n]",
  perl: (letter, inClass) => (inClass && letter === "w" ? NATIVE_WORD : NATIVE_PERL[letter]),
  property: nativeProperty,
  boundary: (negated) =>
    negated
      ? `(?:(?<=[${NATIVE_WORD}])(?=[${NATIVE_WORD}])|(?<![${NATIVE_WORD}])(?![${NATIVE_WORD}]))`
      : `(?:(?<=[${NATIVE_WORD}])(?![${NATIVE_WORD}])|(?<![${NATIVE_WORD}])(?=[${NATIVE_WORD}]))`,
  anchor: (kind) => `(?:${kind})`,
  groupOpen: "(?:",
};

function quantifier(node: { min: number; max: number | null; form: string }): string {
  switch (node.form) {
    case "?":
    case "*":
    case "+":
      return node.form;
    case "exact":
      return `{${node.min}}`;
    default:
      return `{${node.min},${node.max ?? ""}}`;
  }
}

function renderItem(item: ClassItem, dialect: Dialect): string {
  switch (item.kind) {
    case "literal":
      return dialect.char(item.char, true);
    case "range":
      return `${dialect.char(item.from, true)}-${dialect.char(item.to, true)}`;
    case "perl":
      return dialect.perl(item.letter, true);
    case "property":
      return dialect.property(item.name, item.negated);
  }
}

function renderNode(node: Node, dialect: Dialect): string {
  switch (node.kind) {
    case "literal":
      return dialect.char(node.char, false);
    case "any":
      return dialect.any;
    case "perl":
      return dialect.perl(node.letter, false);
    case "property":
      return dialect.property(node.name, node.negated);
    case "class":
      return `[${node.negated ? "^" : ""}${node.items.map((item) => renderItem(item, dialect)).join("")}]`;
    case "start":
      return dialect.anchor("^");
    case "end":
      return dialect.anchor("$");
    case "boundary":
      return dialect.boundary(node.negated);
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

const code = (character: string): number => character.codePointAt(0)!;

const LETTERS = [..."aaabbbccd"].map(code);
const OTHERS = [..."1_ ٥كتب😀𝔸é"].map(code);
const PUNCTUATION = [..."*.+?()[]{}|^$\\-/!,"].map(code);
const LOW = [0x01, 0x1f, 0x7f, 0x80, 0xff, 0x100, 0x2028, 0xfeff, 0xffff, 0x10000, 0x10ffff];
const CONTROLS = [10, 9, 13, 12, 11];
const RANGES = [
  ["a", "c"],
  ["a", "z"],
  ["b", "d"],
  ["0", "9"],
  ["ا", "ي"],
  ["ك", "ي"],
  ["\u{1F600}", "\u{1F64F}"],
  [" ", "/"],
].map(([from, to]): readonly [number, number] => [code(from), code(to)]);
const PERL_LETTERS: readonly PerlLetter[] = ["d", "D", "w", "W", "s", "S"];
const PROPERTIES = ["L", "Lu", "Ll", "N", "Nd", "P", "S", "Z", "Arabic", "Latin", "Greek", "Han"];
const SIMPLE = { "?": { min: 0, max: 1 }, "*": { min: 0, max: null }, "+": { min: 1, max: null } };

const NOISE = [..."abc1٥ك_ -😀𝔸é"];
const NOISE_WITH_CONTROLS = [...NOISE, "\n", "\t", "\r", " "];

const MAX_DEPTH = 3;

function char(random: Random, scope: Scope, value: number): Char {
  if (scope === "unchanged") return { code: value, form: PUNCTUATION.includes(value) && random.chance(0.3) ? "backslash" : "plain" };
  const forms: [number, Form][] = [[8, "plain"]];
  if (CONTROLS.includes(value)) forms.push([4, "control"]);
  else if (PUNCTUATION.includes(value)) forms.push([2, "backslash"]);
  if (value <= 0xff) forms.push([1, "hex2"]);
  forms.push([1, "hexBraced"]);
  if (value <= 0xffff) forms.push([1, "unicode4"]);
  const form = random.weighted(forms);
  const printable = value >= 0x20 && value !== 0x7f && value !== 0x2028 && value !== 0xfeff && !(value >= 0x80 && value < 0xa0);
  return { code: value, form: form === "plain" && !printable ? "hexBraced" : form };
}

function literalCode(random: Random, scope: Scope): number {
  const pools = [[10, LETTERS], [4, OTHERS], [2, PUNCTUATION]] as [number, number[]][];
  if (scope === "all") pools.push([2, CONTROLS], [1, LOW]);
  return random.pick(random.weighted(pools));
}

function classItem(random: Random, scope: Scope): ClassItem {
  if (random.chance(0.35)) {
    const [from, to] = random.pick(RANGES);
    return { kind: "range", from: char(random, scope, from), to: char(random, scope, to) };
  }
  if (random.chance(0.3)) return { kind: "perl", letter: random.pick(PERL_LETTERS) };
  if (scope === "all" && random.chance(0.15)) return { kind: "property", name: random.pick(PROPERTIES), negated: random.chance(0.4) };
  return { kind: "literal", char: char(random, scope, literalCode(random, scope)) };
}

function characterClass(random: Random, scope: Scope): Node {
  const items: ClassItem[] = [];
  for (let k = random.between(1, 4); k > 0; k--) items.push(classItem(random, scope));
  return { kind: "class", negated: random.chance(0.3), items };
}

type AtomKind = "literal" | "any" | "perl" | "class" | "start" | "end" | "group" | "property" | "boundary";

function atom(random: Random, scope: Scope, depth: number): Node {
  const all = scope === "all";
  const choice = random.weighted<AtomKind>([
    [40, "literal"],
    [6, "any"],
    [8, "perl"],
    [12, "class"],
    [5, "start"],
    [5, "end"],
    [depth < MAX_DEPTH ? 12 : 0, "group"],
    [all ? 5 : 0, "property"],
    [all ? 6 : 0, "boundary"],
  ]);
  switch (choice) {
    case "literal":
      return { kind: "literal", char: char(random, scope, literalCode(random, scope)) };
    case "any":
      return { kind: "any" };
    case "perl":
      return { kind: "perl", letter: random.pick(PERL_LETTERS) };
    case "class":
      return characterClass(random, scope);
    case "start":
      return { kind: "start" };
    case "end":
      return { kind: "end" };
    case "property":
      return { kind: "property", name: random.pick(PROPERTIES), negated: random.chance(0.4) };
    case "boundary":
      return { kind: "boundary", negated: random.chance(0.4) };
    case "group":
      return { kind: "group", body: expression(random, scope, depth + 1) };
  }
}

function bounds(random: Random, large: boolean): { min: number; max: number | null } {
  const top = large && random.chance(0.15) ? 100 : random.chance(0.8) ? 3 : 8;
  const min = random.between(0, top);
  return { min, max: random.chance(0.2) ? null : random.between(min, Math.min(top, min + 6)) };
}

function piece(random: Random, scope: Scope, depth: number): Node {
  const body = atom(random, scope, depth);
  const empty = body.kind === "start" || body.kind === "end" || body.kind === "boundary";
  if (empty && scope === "unchanged") return body;
  const form = random.weighted<"none" | "?" | "*" | "+" | "range">([
    [55, "none"],
    [8, "?"],
    [8, "*"],
    [8, "+"],
    [21, "range"],
  ]);
  if (form === "none") return body;
  if (form !== "range") return { kind: "repeat", body, form, ...SIMPLE[form] };
  const { min, max } = bounds(random, depth === 0 && body.kind !== "group");
  return { kind: "repeat", body, min, max, form: scope === "all" && max === min && random.chance(0.5) ? "exact" : "range" };
}

function expression(random: Random, scope: Scope, depth: number): Expression {
  const branches: Expression = [];
  for (let b = random.weighted([[60, 1], [30, 2], [10, 3]]); b > 0; b--) {
    const sequence: Node[] = [];
    for (let n = random.weighted([[10, 0], [30, 1], [30, 2], [20, 3], [10, 4]]); n > 0; n--) sequence.push(piece(random, scope, depth));
    branches.push(sequence);
  }
  return branches;
}

export function generate(random: Random, scope: Scope, maxLength: number): Expression {
  for (;;) {
    const candidate = expression(random, scope, 0);
    if (renderEngine(candidate).length <= maxLength) return candidate;
  }
}

function charsOf(expr: Expression, found: Set<string>): void {
  const add = (entry: Char) => found.add(text(entry.code));
  for (const branch of expr) {
    for (const node of branch) {
      const target = node.kind === "repeat" ? node.body : node;
      if (target.kind === "literal") add(target.char);
      else if (target.kind === "class") {
        for (const item of target.items) {
          if (item.kind === "literal") add(item.char);
          else if (item.kind === "range") [item.from, item.to].forEach(add);
        }
      } else if (target.kind === "group") charsOf(target.body, found);
    }
  }
}

export function textsFor(random: Random, scope: Scope, expr: Expression, count: number): string[] {
  const noise = scope === "all" ? NOISE_WITH_CONTROLS : NOISE;
  const own = new Set<string>();
  charsOf(expr, own);
  const pool = own.size === 0 ? noise : [...own];
  const texts = [""];
  while (texts.length < count) {
    const length = random.chance(0.85) ? random.between(0, 7) : random.between(8, 24);
    let built = "";
    for (let k = 0; k < length; k++) built += random.chance(0.75) ? random.pick(pool) : random.pick(noise);
    texts.push(built);
  }
  return texts;
}
