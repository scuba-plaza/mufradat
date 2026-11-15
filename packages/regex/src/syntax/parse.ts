import { MAX_CODE_POINT } from "../program/ranges";
import { hasProperty } from "../unicode";
import type { ClassItem, ErrorCode, Limits, Node, RegexError } from "./types";

const NEWLINE = 10;
const CONTROL: Record<string, number> = { n: 10, t: 9, r: 13, f: 12, v: 11 };
const isHex = (char: string | undefined): char is string => char !== undefined && /^[0-9a-fA-F]$/.test(char);
const isNameChar = (char: string | undefined): char is string => char !== undefined && /^[0-9A-Za-z_]$/.test(char);
const isPunctuation = (char: string) => char < "\u0080" && !/[0-9A-Za-z]/.test(char);

export const DEFAULT_LIMITS: Limits = { maxLength: 200, maxRepeat: 100, maxDepth: 1000, maxInstructions: 20_000 };

export type Parsed = { tree: Node } | { error: RegexError };

interface Operator {
  min: number;
  max: number | null;
  length: number;
}

class Refusal extends Error {
  constructor(readonly error: RegexError) {
    super(error.code);
  }
}

function refuse(code: ErrorCode, position?: number, detail?: string): never {
  throw new Refusal({ code, position, detail });
}

class Parser {
  private at = 0;
  private depth = 0;

  constructor(
    private readonly chars: string[],
    private readonly limits: Limits,
  ) {}

  parse(): Node {
    const tree = this.alternation();
    if (this.at < this.chars.length) refuse("unexpected_paren", this.at);
    return tree;
  }

  private alternation(): Node {
    const branches = [this.concatenation()];
    while (this.chars[this.at] === "|") {
      this.at++;
      branches.push(this.concatenation());
    }
    return branches.length === 1 ? branches[0] : { kind: "alternate", branches };
  }

  private concatenation(): Node {
    const items: Node[] = [];
    while (this.at < this.chars.length && this.chars[this.at] !== "|" && this.chars[this.at] !== ")") items.push(this.repetition());
    return items.length === 1 ? items[0] : { kind: "concat", items };
  }

  private repetition(): Node {
    this.refuseLeadingOperator();
    const body = this.atom();
    const start = this.at;
    const operator = this.operator();
    if (!operator) return body;
    this.checkBounds(operator, start);
    this.at += operator.length;
    const next = this.chars[this.at];
    if (next === "?" || next === "+") refuse("unsupported", start, this.chars.slice(start, this.at + 1).join(""));
    if (next === "*" || (next === "{" && this.operator())) refuse("nested_repeat", start);
    return { kind: "repeat", body, min: operator.min, max: operator.max };
  }

  private refuseLeadingOperator(): void {
    const char = this.chars[this.at];
    if (char === "?" || char === "*" || char === "+" || (char === "{" && this.operator())) refuse("missing_repeat_argument", this.at);
  }

  private operator(): Operator | null {
    const char = this.chars[this.at];
    if (char === "?") return { min: 0, max: 1, length: 1 };
    if (char === "*") return { min: 0, max: null, length: 1 };
    if (char === "+") return { min: 1, max: null, length: 1 };
    if (char !== "{") return null;
    return this.counted() ?? refuse("invalid_repeat", this.at);
  }

  private counted(): Operator | null {
    let end = this.at + 1;
    const number = (): number | null => {
      const from = end;
      while (this.chars[end] >= "0" && this.chars[end] <= "9") end++;
      return end === from ? null : Number(this.chars.slice(from, end).join(""));
    };
    const min = number();
    if (min === null) return null;
    let max: number | null = min;
    if (this.chars[end] === ",") {
      end++;
      max = number();
    }
    return this.chars[end] === "}" ? { min, max, length: end + 1 - this.at } : null;
  }

  private checkBounds({ min, max }: Operator, start: number): void {
    const limit = this.limits.maxRepeat;
    if (min > limit || (max !== null && max > limit)) refuse("repeat_too_large", start);
    if (max !== null && max < min) refuse("reversed_repeat", start);
  }

  private atom(): Node {
    const start = this.at++;
    const char = this.chars[start];
    if (char === "(") return this.group(start);
    if (char === "[") return this.characterClass(start);
    if (char === ".") return { kind: "class", negated: true, items: [{ kind: "range", from: NEWLINE, to: NEWLINE }] };
    if (char === "^") return { kind: "assert", assertion: "start" };
    if (char === "$") return { kind: "assert", assertion: "end" };
    if (char === "\\") return this.escapedAtom();
    return { kind: "literal", code: char.codePointAt(0)! };
  }

  private escapedAtom(): Node {
    const next = this.chars[this.at];
    if (next === "b" || next === "B") {
      this.at++;
      return { kind: "assert", assertion: next === "b" ? "wordBoundary" : "notWordBoundary" };
    }
    const escaped = this.escape();
    return typeof escaped === "number" ? { kind: "literal", code: escaped } : { kind: "class", negated: false, items: [escaped] };
  }

  private escape(): number | ClassItem {
    const start = this.at - 1;
    const char = this.chars[this.at++];
    if (char === undefined) return refuse("trailing_backslash", start);
    if (char in CONTROL) return CONTROL[char];
    if (/^[dws]$/i.test(char)) return { kind: "perl", name: char.toLowerCase() as "d" | "w" | "s", negated: char < "a" };
    if (char === "p" || char === "P") return this.property(start, char === "P");
    if (char === "x") return this.chars[this.at] === "{" ? this.bracedHex(start) : this.hexDigits(start, 2);
    if (char === "u") return this.hexDigits(start, 4);
    if (isPunctuation(char)) return char.codePointAt(0)!;
    return this.badEscape(start);
  }

  private property(start: number, negated: boolean): ClassItem {
    if (this.chars[this.at] !== "{") {
      if (this.at < this.chars.length) this.at++;
      return this.badProperty(start);
    }
    this.at++;
    const from = this.at;
    while (isNameChar(this.chars[this.at])) this.at++;
    if (this.at === from || this.chars[this.at] !== "}") return this.badProperty(start);
    const name = this.chars.slice(from, this.at).join("");
    this.at++;
    if (!hasProperty(name)) return this.badProperty(start);
    return { kind: "property", name, negated };
  }

  private badProperty(start: number): never {
    return refuse("invalid_property", start, this.chars.slice(start, this.at).join(""));
  }

  private hexDigits(start: number, count: number): number {
    let value = 0;
    for (let k = 0; k < count; k++) {
      const digit = this.chars[this.at];
      if (!isHex(digit)) return this.badEscape(start, this.at + 1);
      value = value * 16 + parseInt(digit, 16);
      this.at++;
    }
    return value;
  }

  private bracedHex(start: number): number {
    this.at++;
    let value = 0;
    let digits = 0;
    while (isHex(this.chars[this.at])) {
      if (value <= MAX_CODE_POINT) value = value * 16 + parseInt(this.chars[this.at], 16);
      digits++;
      this.at++;
    }
    if (digits === 0 || value > MAX_CODE_POINT || this.chars[this.at] !== "}") return this.badEscape(start, this.at + 1);
    this.at++;
    return value;
  }

  private badEscape(start: number, end = this.at): never {
    return refuse("invalid_escape", start, this.chars.slice(start, end).join(""));
  }

  private characterClass(start: number): Node {
    const negated = this.chars[this.at] === "^";
    if (negated) this.at++;
    const items: ClassItem[] = [];
    for (;;) {
      if (this.at >= this.chars.length) refuse("missing_bracket", start);
      if (this.chars[this.at] === "]") break;
      this.refusePosixClass();
      items.push(this.classItem());
    }
    this.at++;
    if (items.length === 0) refuse("empty_class", start);
    return { kind: "class", negated, items };
  }

  private refusePosixClass(): void {
    if (this.chars[this.at] !== "[" || this.chars[this.at + 1] !== ":") return;
    for (let end = this.at + 2; end + 1 < this.chars.length; end++) {
      if (this.chars[end] === ":" && this.chars[end + 1] === "]") refuse("unsupported", this.at, this.chars.slice(this.at, end + 2).join(""));
    }
  }

  private classItem(): ClassItem {
    const start = this.at;
    const from = this.classMember();
    if (typeof from !== "number") return from;
    const dash = this.chars[this.at] === "-" && this.at + 1 < this.chars.length && this.chars[this.at + 1] !== "]";
    if (!dash) return { kind: "range", from, to: from };
    this.at++;
    const to = this.classMember();
    if (typeof to !== "number") return refuse("invalid_char_range", start);
    if (to < from) return refuse("reversed_char_range", start);
    return { kind: "range", from, to };
  }

  private classMember(): number | ClassItem {
    const char = this.chars[this.at++];
    return char === "\\" ? this.escape() : char.codePointAt(0)!;
  }

  private group(start: number): Node {
    if (this.chars[this.at] === "?") refuse("unsupported", start, this.chars.slice(start, start + 3).join(""));
    if (++this.depth > this.limits.maxDepth) refuse("nesting_depth", start);
    const inner = this.alternation();
    if (this.chars[this.at] !== ")") refuse("missing_paren", start);
    this.at++;
    this.depth--;
    return inner;
  }
}

export function parse(pattern: string, limits: Limits = DEFAULT_LIMITS): Parsed {
  if (pattern.length > limits.maxLength) return { error: { code: "too_long" } };
  try {
    return { tree: new Parser(Array.from(pattern), limits).parse() };
  } catch (failure) {
    if (failure instanceof Refusal) return { error: failure.error };
    throw failure;
  }
}
