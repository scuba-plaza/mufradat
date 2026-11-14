import type { ClassItem, Node } from "../../src/syntax/types";

type Side = "edge" | "word" | "other";

type R = { id: number } & (
  | { kind: "none" }
  | { kind: "empty" }
  | { kind: "set"; test: (code: number) => boolean }
  | { kind: "cat"; left: R; right: R }
  | { kind: "alt"; parts: R[] }
  | { kind: "rep"; body: R; min: number; max: number | null }
);

const WORD = /[\p{L}\p{N}_]/u;
const PERL = { d: /\p{Nd}/u, w: WORD, s: /\s/u };
const properties = new Map<string, RegExp>();

function property(name: string): RegExp {
  let found = properties.get(name);
  if (!found) {
    found = new RegExp(`\\p{${/^[A-Z][a-z]?$/.test(name) ? "" : "Script="}${name}}`, "u");
    properties.set(name, found);
  }
  return found;
}

function itemHolds(item: ClassItem, code: number): boolean {
  if (item.kind === "range") return code >= item.from && code <= item.to;
  const base = item.kind === "perl" ? PERL[item.name] : property(item.name);
  return base.test(String.fromCodePoint(code)) !== item.negated;
}

const sideOf = (code: number | null): Side => (code === null ? "edge" : WORD.test(String.fromCodePoint(code)) ? "word" : "other");

export function referenceMatcher(tree: Node): (text: string) => boolean {
  const table = new Map<string, R>();
  const intern = (key: string, build: (id: number) => R): R => {
    let found = table.get(key);
    if (!found) {
      found = build(table.size);
      table.set(key, found);
    }
    return found;
  };

  const NONE = intern("none", (id) => ({ id, kind: "none" }));
  const EMPTY = intern("empty", (id) => ({ id, kind: "empty" }));
  const set = (key: string, test: (code: number) => boolean) => intern(`set:${key}`, (id) => ({ id, kind: "set", test }));

  const cat = (left: R, right: R): R => {
    if (left === NONE || right === NONE) return NONE;
    if (left === EMPTY) return right;
    if (right === EMPTY) return left;
    if (left.kind === "cat") return cat(left.left, cat(left.right, right));
    return intern(`cat:${left.id},${right.id}`, (id) => ({ id, kind: "cat", left, right }));
  };

  const alt = (...choices: R[]): R => {
    const parts = new Map<number, R>();
    for (const choice of choices) for (const part of choice.kind === "alt" ? choice.parts : [choice]) if (part !== NONE) parts.set(part.id, part);
    const sorted = [...parts.values()].sort((a, b) => a.id - b.id);
    if (sorted.length === 0) return NONE;
    if (sorted.length === 1) return sorted[0];
    return intern(`alt:${sorted.map((part) => part.id).join(",")}`, (id) => ({ id, kind: "alt", parts: sorted }));
  };

  const rep = (body: R, min: number, max: number | null): R => {
    if (max === 0 || body === EMPTY) return EMPTY;
    if (body === NONE) return min === 0 ? EMPTY : NONE;
    return intern(`rep:${body.id},${min},${max}`, (id) => ({ id, kind: "rep", body, min, max }));
  };

  const lower = (node: Node): R => {
    switch (node.kind) {
      case "literal":
        return set(`lit:${node.code}`, (code) => code === node.code);
      case "class":
        return set(`class:${JSON.stringify(node)}`, (code) => node.items.some((item) => itemHolds(item, code)) !== node.negated);
      case "concat":
        return node.items.reduceRight((rest, item) => cat(lower(item), rest), EMPTY);
      case "alternate":
        return alt(...node.branches.map(lower));
      case "repeat":
        return rep(lower(node.body), node.min, node.max);
    }
  };

  const nullable = (r: R, before: Side, after: Side): boolean => {
    switch (r.kind) {
      case "none":
      case "set":
        return false;
      case "empty":
        return true;
      case "cat":
        return nullable(r.left, before, after) && nullable(r.right, before, after);
      case "alt":
        return r.parts.some((part) => nullable(part, before, after));
      case "rep":
        return r.min === 0 || nullable(r.body, before, after);
    }
  };

  const derive = (r: R, code: number, before: Side): R => {
    switch (r.kind) {
      case "none":
      case "empty":
        return NONE;
      case "set":
        return r.test(code) ? EMPTY : NONE;
      case "cat": {
        const first = cat(derive(r.left, code, before), r.right);
        return nullable(r.left, before, sideOf(code)) ? alt(first, derive(r.right, code, before)) : first;
      }
      case "alt":
        return alt(...r.parts.map((part) => derive(part, code, before)));
      case "rep": {
        const rest = rep(r.body, Math.max(r.min - 1, 0), r.max === null ? null : r.max - 1);
        const first = cat(derive(r.body, code, before), rest);
        const skips = nullable(r.body, before, sideOf(code)) && !(r.min === 0 && r.max === null);
        return skips ? alt(first, derive(rest, code, before)) : first;
      }
    }
  };

  const anywhere = set("any", () => true);
  const root = cat(rep(anywhere, 0, null), lower(tree));

  return (text) => {
    const codes = Array.from(text, (char) => char.codePointAt(0)!);
    let state = root;
    let before: Side = "edge";
    for (let at = 0; ; at++) {
      if (nullable(state, before, at < codes.length ? sideOf(codes[at]) : "edge")) return true;
      if (at === codes.length) return false;
      state = derive(state, codes[at], before);
      before = sideOf(codes[at]);
    }
  };
}
