import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { saveBody } from "./upload";

const dirs: string[] = [];

afterEach(() => {
  for (const dir of dirs.splice(0)) fs.rmSync(dir, { recursive: true, force: true });
});

const streamOf = (chunks: number[][]) =>
  new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(new Uint8Array(chunk));
      controller.close();
    },
  });

describe("saving an uploaded body", () => {
  const file = () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-upload-"));
    dirs.push(dir);
    return path.join(dir, "body");
  };

  it("writes every chunk in order", async () => {
    const target = file();
    await saveBody(streamOf([[1, 2], [3], [4, 5, 6]]), target, 100);
    expect([...fs.readFileSync(target)]).toEqual([1, 2, 3, 4, 5, 6]);
  });

  it("refuses a body that grows past the limit while it streams, saying how large a file may be", async () => {
    await expect(saveBody(streamOf([[1, 2, 3], [4, 5, 6]]), file(), 5)).rejects.toThrow("That file is larger than 3 GiB.");
  });

  it("leaves no file for a later cleanup to miss when the body fails at once", async () => {
    const target = file();
    await expect(saveBody(streamOf([[1, 2, 3, 4, 5, 6]]), target, 5)).rejects.toThrow();
    fs.rmSync(target, { force: true });
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(fs.existsSync(target)).toBe(false);
  });
});
