import type { ClassItem, Limits, Node, RegexError } from "../syntax/types";
import { PERL, propertyRanges } from "../unicode";
import { negate, normalize, type Ranges } from "./ranges";
import { size } from "./size";
import type { Inst } from "./types";

const PLACEHOLDER = -1;

type Repeat = Extract<Node, { kind: "repeat" }>;

function itemRanges(item: ClassItem): Ranges {
  if (item.kind === "range") return [item.from, item.to];
  const base = item.kind === "perl" ? PERL[item.name] : propertyRanges(item.name);
  return item.negated ? negate(base) : base;
}

function classRanges(items: ClassItem[], negated: boolean): number[] {
  const merged = normalize(items.flatMap(itemRanges));
  return negated ? negate(merged) : merged;
}

class Emitter {
  readonly program: Inst[] = [];
  private readonly sets = new Map<Node, number[]>();

  emit(node: Node): void {
    switch (node.kind) {
      case "literal":
        this.program.push({ op: "ranges", ranges: [node.code, node.code] });
        return;
      case "class":
        this.program.push({ op: "ranges", ranges: this.setOf(node) });
        return;
      case "assert":
        this.program.push({ op: "assert", assertion: node.assertion });
        return;
      case "concat":
        for (const item of node.items) this.emit(item);
        return;
      case "alternate":
        this.alternate(node.branches);
        return;
      case "repeat":
        this.repeat(node);
        return;
    }
  }

  private setOf(node: Extract<Node, { kind: "class" }>): number[] {
    let ranges = this.sets.get(node);
    if (!ranges) {
      ranges = classRanges(node.items, node.negated);
      this.sets.set(node, ranges);
    }
    return ranges;
  }

  private patch(at: number, target: number): void {
    const inst = this.program[at];
    if (inst.op === "split") inst.second = target;
    else if (inst.op === "jump") inst.target = target;
  }

  private alternate(branches: Node[]): void {
    const exits: number[] = [];
    const last = branches.length - 1;
    for (let index = 0; index < last; index++) {
      const choice = this.program.length;
      this.program.push({ op: "split", first: choice + 1, second: PLACEHOLDER });
      this.emit(branches[index]);
      exits.push(this.program.length);
      this.program.push({ op: "jump", target: PLACEHOLDER });
      this.patch(choice, this.program.length);
    }
    this.emit(branches[last]);
    for (const at of exits) this.patch(at, this.program.length);
  }

  private repeat({ body, min, max }: Repeat): void {
    if (max === null) {
      if (min === 0) this.star(body);
      else this.plus(body, min);
      return;
    }
    for (let copy = 0; copy < min; copy++) this.emit(body);
    const choices: number[] = [];
    for (let copy = min; copy < max; copy++) {
      choices.push(this.program.length);
      this.program.push({ op: "split", first: this.program.length + 1, second: PLACEHOLDER });
      this.emit(body);
    }
    for (const at of choices) this.patch(at, this.program.length);
  }

  private star(body: Node): void {
    const start = this.program.length;
    this.program.push({ op: "split", first: start + 1, second: PLACEHOLDER });
    this.emit(body);
    this.program.push({ op: "jump", target: start });
    this.patch(start, this.program.length);
  }

  private plus(body: Node, min: number): void {
    for (let copy = 1; copy < min; copy++) this.emit(body);
    const start = this.program.length;
    this.emit(body);
    this.program.push({ op: "split", first: start, second: this.program.length + 1 });
  }
}

export function compileProgram(tree: Node, limits: Limits): { program: Inst[] } | { error: RegexError } {
  if (size(tree) + 1 > limits.maxInstructions) return { error: { code: "too_big" } };
  const emitter = new Emitter();
  emitter.emit(tree);
  emitter.program.push({ op: "match" });
  return { program: emitter.program };
}
