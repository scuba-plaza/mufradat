import type { ClassItem, ErrorCode, Limits, Node, RegexError } from "./types";

const NEWLINE = 10;

export const DEFAULT_LIMITS: Limits = { maxLength: 200, maxDepth: 1000 };

export type Parsed = { tree: Node } | { error: RegexError };

class Refusal extends Error {
  constructor(readonly error: RegexError) {
    super(error.code);
  }
}

const refuse = (code: ErrorCode, position?: number, detail?: string): never => {
  throw new Refusal({ code, position, detail });
};

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
    while (this.at < this.chars.length && this.chars[this.at] !== "|" && this.chars[this.at] !== ")") items.push(this.atom());
    return items.length === 1 ? items[0] : { kind: "concat", items };
  }

  private atom(): Node {
    const start = this.at++;
    const char = this.chars[start];
    if (char === "(") return this.group(start);
    if (char === "[") return this.characterClass(start);
    if (char === ".") return { kind: "class", negated: true, items: [{ kind: "range", from: NEWLINE, to: NEWLINE }] };
    return { kind: "literal", code: char.codePointAt(0)! };
  }

  private characterClass(start: number): Node {
    const negated = this.chars[this.at] === "^";
    if (negated) this.at++;
    const items: ClassItem[] = [];
    for (;;) {
      if (this.at >= this.chars.length) refuse("missing_bracket", start);
      if (this.chars[this.at] === "]") break;
      items.push(this.classItem());
    }
    this.at++;
    if (items.length === 0) refuse("empty_class", start);
    return { kind: "class", negated, items };
  }

  private classItem(): ClassItem {
    const start = this.at;
    const from = this.classCode();
    const dash = this.chars[this.at] === "-" && this.at + 1 < this.chars.length && this.chars[this.at + 1] !== "]";
    if (!dash) return { kind: "range", from, to: from };
    this.at++;
    const to = this.classCode();
    if (to < from) refuse("reversed_char_range", start);
    return { kind: "range", from, to };
  }

  private classCode(): number {
    return this.chars[this.at++].codePointAt(0)!;
  }

  private group(start: number): Node {
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
