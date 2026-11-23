import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, vi } from "vitest";

export const mp3 = Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]);

export const withParams = <P>(params: P) => ({ params: Promise.resolve(params) });

export function tempWorkspace(): () => string {
  let home = "";
  beforeEach(() => {
    home = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-route-"));
    vi.stubEnv("XDG_CONFIG_HOME", path.join(home, "config"));
    vi.stubEnv("MUFRADAT_WORKSPACE", path.join(home, "workspace"));
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    fs.rmSync(home, { recursive: true, force: true });
  });
  return () => home;
}
