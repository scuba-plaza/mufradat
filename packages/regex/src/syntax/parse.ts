import type { ErrorCode, Limits, Node, RegexError } from "./types";

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
    return { kind: "literal", code: char.codePointAt(0)! };
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
