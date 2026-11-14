import { TIMEOUT, within } from "./watchdog";

const BUDGET_MS = 100;

export function nativeMatcher(source: string): (texts: readonly string[]) => (boolean | typeof TIMEOUT)[] {
  const re = new RegExp(source, "vy");
  const search = (text: string): boolean => {
    for (let at = 0; at <= text.length; at += at < text.length && text.codePointAt(at)! > 0xffff ? 2 : 1) {
      re.lastIndex = at;
      if (re.test(text)) return true;
    }
    return false;
  };
  return (texts) => {
    const all = within(BUDGET_MS, () => texts.map(search));
    if (all !== TIMEOUT) return all;
    return texts.map((text) => within(BUDGET_MS, () => search(text)));
  };
}
