/** A compiled search pattern: `matches` is true when the pattern occurs in the text. */
export interface Pattern {
  matches(text: string): boolean;
}

type Inst =
  | { t: "char"; set: CharSet }
  | { t: "split"; a: number; b: number }
  | { t: "jump"; a: number }
  | { t: "assert"; kind: "start" | "end" }
  | { t: "match" };

interface CharSet {
  holds(code: number): boolean;
}

const literal = (code: number): CharSet => ({ holds: (c) => c === code });
const ANY: CharSet = { holds: () => true };
const not = (set: CharSet): CharSet => ({ holds: (c) => !set.holds(c) });
const perl = (test: (ch: string) => boolean): CharSet => ({ holds: (c) => test(String.fromCodePoint(c)) });
const DIGIT = perl((ch) => /\p{Nd}/u.test(ch));
const WORD = perl((ch) => /[\p{L}\p{N}_]/u.test(ch));
const SPACE = perl((ch) => /\s/u.test(ch));
const PERL: Record<string, CharSet> = {
  d: DIGIT,
  w: WORD,
  s: SPACE,
  D: not(DIGIT),
  W: not(WORD),
  S: not(SPACE),
};

const MAX_LENGTH = 200;
const MAX_INSTS = 20_000;
const MAX_REPEAT = 100;
const TOO_BIG = "The pattern is too big to search with.";

const shifted = (insts: Inst[], by: number): Inst[] =>
  insts.map((inst) =>
    inst.t === "split" ? { t: "split", a: inst.a + by, b: inst.b + by } : inst.t === "jump" ? { t: "jump", a: inst.a + by } : inst,
  );

const optional = (x: Inst[]): Inst[] => [{ t: "split", a: 1, b: x.length + 1 }, ...shifted(x, 1)];
const star = (x: Inst[]): Inst[] => [{ t: "split", a: 1, b: x.length + 2 }, ...shifted(x, 1), { t: "jump", a: 0 }];
const plus = (x: Inst[]): Inst[] => [...x, { t: "split", a: 0, b: x.length + 1 }];

/** Appends a self-contained piece to a program, moving its jumps to where it lands. */
const embed = (program: Inst[], piece: Inst[]) => {
  for (const inst of shifted(piece, program.length)) program.push(inst);
};

const repeated = (x: Inst[], from: number, to: number | null): Inst[] => {
  const out: Inst[] = [];
  for (let k = 0; k < from; k++) embed(out, x);
  if (to === null) embed(out, star(x));
  else for (let k = from; k < to; k++) embed(out, optional(x));
  return out;
};

class Compiler {
  i = 0;
  private chars: string[];

  constructor(pattern: string) {
    this.chars = Array.from(pattern);
  }

  compile(): Inst[] | string {
    const alt = this.alternation();
    if (typeof alt === "string") return alt;
    if (this.i < this.chars.length) return "A ) has no matching (.";
    return [...alt, { t: "match" }];
  }

  private peek(): string | undefined {
    return this.chars[this.i];
  }

  private take(): string {
    return this.chars[this.i++];
  }

  private alternation(): Inst[] | string {
    let acc = this.sequence();
    if (typeof acc === "string") return acc;
    while (this.peek() === "|") {
      this.take();
      const next = this.sequence();
      if (typeof next === "string") return next;
      const merged: Inst[] = [{ t: "split", a: 1, b: acc.length + 2 }];
      embed(merged, acc);
      merged.push({ t: "jump", a: acc.length + 2 + next.length });
      embed(merged, next);
      if (merged.length > MAX_INSTS) return TOO_BIG;
      acc = merged;
    }
    return acc;
  }

  private braces(): { from: number; to: number | null; length: number } | null {
    if (this.peek() !== "{") return null;
    const found = /^\{(\d*),(\d*)\}/.exec(this.chars.slice(this.i).join(""));
    if (!found) return null;
    return { from: found[1] === "" ? 0 : Number(found[1]), to: found[2] === "" ? null : Number(found[2]), length: found[0].length };
  }

  private quantifierAhead(): boolean {
    const ch = this.peek();
    return ch === "?" || ch === "*" || ch === "+" || this.braces() !== null;
  }

  private sequence(): Inst[] | string {
    const out: Inst[] = [];
    while (this.peek() !== undefined && this.peek() !== "|" && this.peek() !== ")") {
      if (this.quantifierAhead()) return "A quantifier has nothing before it.";
      const piece = this.repeat();
      if (typeof piece === "string") return piece;
      embed(out, piece);
      if (out.length > MAX_INSTS) return TOO_BIG;
    }
    return out;
  }

  private repeat(): Inst[] | string {
    const atom = this.atom();
    if (typeof atom === "string") return atom;
    const ch = this.peek();
    if (ch === "?") {
      this.take();
      return optional(atom);
    }
    if (ch === "*") {
      this.take();
      return star(atom);
    }
    if (ch === "+") {
      this.take();
      return plus(atom);
    }
    const braces = this.braces();
    if (!braces) return atom;
    this.i += braces.length;
    const { from, to } = braces;
    if (from > MAX_REPEAT || (to !== null && to > MAX_REPEAT)) return "A quantifier repeats more than 100 times.";
    if (to !== null && to < from) return "A quantifier goes from the smaller to the larger number.";
    if (atom.length * ((to ?? from) + 1) > MAX_INSTS) return TOO_BIG;
    return repeated(atom, from, to);
  }

  private classItem(): { set: CharSet; code: number | null } | string {
    const ch = this.take();
    if (ch === "\\") {
      const escaped = this.take();
      if (escaped === undefined) return "The pattern ends with a lone backslash.";
      const set = PERL[escaped];
      if (set) return { set, code: null };
      return { set: literal(escaped.codePointAt(0)!), code: escaped.codePointAt(0)! };
    }
    return { set: literal(ch.codePointAt(0)!), code: ch.codePointAt(0)! };
  }

  private charClass(): CharSet | string {
    const negate = this.peek() === "^" ? (this.take(), true) : false;
    const members: CharSet[] = [];
    while (this.peek() !== "]") {
      if (this.peek() === undefined) return "A class in [] is not closed.";
      const first = this.classItem();
      if (typeof first === "string") return first;
      const after = this.chars[this.i + 1];
      const dash = this.peek() === "-" && after !== "]" && after !== undefined;
      if (first.code !== null && dash) {
        this.take();
        const last = this.classItem();
        if (typeof last === "string") return last;
        if (last.code === null) return "A range in a class needs one character on each side of the dash.";
        if (last.code < first.code) return "A range in a class goes from the smaller to the larger character.";
        const [from, to] = [first.code, last.code];
        members.push({ holds: (code) => code >= from && code <= to });
        continue;
      }
      members.push(first.set);
    }
    this.take();
    if (members.length === 0) return "A class in [] is empty.";
    const holds = (code: number) => members.some((m) => m.holds(code));
    return negate ? { holds: (code) => !holds(code) } : { holds };
  }

  private atom(): Inst[] | string {
    const ch = this.take();
    if (ch === "[") {
      const set = this.charClass();
      if (typeof set === "string") return set;
      return [{ t: "char", set }];
    }
    if (ch === "^" || ch === "$") return [{ t: "assert", kind: ch === "^" ? "start" : "end" }];
    if (ch === "(") {
      const group = this.alternation();
      if (typeof group === "string") return group;
      if (this.peek() !== ")") return "A group in () is not closed.";
      this.take();
      return group;
    }
    if (ch === "\\") {
      const escaped = this.take();
      if (escaped === undefined) return "The pattern ends with a lone backslash.";
      return [{ t: "char", set: PERL[escaped] ?? literal(escaped.codePointAt(0)!) }];
    }
    return [{ t: "char", set: ch === "." ? ANY : literal(ch.codePointAt(0)!) }];
  }
}

export function compilePattern(pattern: string): Pattern | string {
  if (pattern.length > MAX_LENGTH) return "The pattern is too long.";
  const compiled = new Compiler(pattern).compile();
  if (typeof compiled === "string") return compiled;
  const insts: Inst[] = [
    { t: "split", a: 1, b: 3 },
    { t: "char", set: ANY },
    { t: "jump", a: 0 },
  ];
  embed(insts, compiled);

  const seen = new Float64Array(insts.length).fill(-1);
  let epoch = 0;

  const matches = (text: string): boolean => {
    const codes = Array.from(text, (ch) => ch.codePointAt(0)!);
    const base = epoch;
    epoch += codes.length + 1;
    let current: number[] = [];
    const closure = (pc: number, at: number, list: number[]): boolean => {
      const stack = [pc];
      while (stack.length > 0) {
        const p = stack.pop()!;
        if (seen[p] === base + at) continue;
        seen[p] = base + at;
        const inst = insts[p];
        if (inst.t === "char") list.push(p);
        else if (inst.t === "split") stack.push(inst.b, inst.a);
        else if (inst.t === "jump") stack.push(inst.a);
        else if (inst.t === "assert") {
          if (inst.kind === "start" ? at === 0 : at === codes.length) stack.push(p + 1);
        } else if (inst.t === "match") return true;
      }
      return false;
    };
    if (closure(0, 0, current)) return true;
    for (let at = 0; at < codes.length; at++) {
      const next: number[] = [];
      for (const pc of current) {
        const inst = insts[pc];
        if (inst.t === "char" && inst.set.holds(codes[at]) && closure(pc + 1, at + 1, next)) return true;
      }
      current = next;
    }
    return false;
  };

  return { matches };
}
