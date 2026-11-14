import { TIMEOUT, within } from "./watchdog";

const BUDGET_MS = 100;

export function nativeMatcher(source: string): (texts: readonly string[]) => (boolean | typeof TIMEOUT)[] {
  const re = new RegExp(source, "v");
  return (texts) => {
    const all = within(BUDGET_MS, () => texts.map((text) => re.test(text)));
    if (all !== TIMEOUT) return all;
    return texts.map((text) => within(BUDGET_MS, () => re.test(text)));
  };
}
