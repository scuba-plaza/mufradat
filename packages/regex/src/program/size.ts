import type { Node } from "../syntax/types";

const CAP = 2 ** 40;

const capped = (value: number): number => Math.min(value, CAP);

export function size(node: Node): number {
  switch (node.kind) {
    case "literal":
    case "class":
    case "assert":
      return 1;
    case "concat":
      return capped(node.items.reduce((total, item) => total + size(item), 0));
    case "alternate":
      return capped(node.branches.reduce((total, branch) => total + size(branch), 2 * (node.branches.length - 1)));
    case "repeat": {
      const body = size(node.body);
      if (node.max === null) return capped(node.min === 0 ? body + 2 : node.min * body + 1);
      return capped(node.min * body + (node.max - node.min) * (body + 1));
    }
  }
}
