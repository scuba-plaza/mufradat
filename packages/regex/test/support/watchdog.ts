import { createContext, runInContext } from "node:vm";

export const TIMEOUT = Symbol("watchdog timed out");

const context = createContext({ run: () => undefined as unknown });

const timedOut = (error: unknown): boolean =>
  typeof error === "object" && error !== null && "code" in error && error.code === "ERR_SCRIPT_EXECUTION_TIMEOUT";

export function within<T>(ms: number, run: () => T): T | typeof TIMEOUT {
  context.run = run;
  try {
    return runInContext("run()", context, { timeout: ms });
  } catch (error) {
    if (timedOut(error)) return TIMEOUT;
    throw error;
  }
}
