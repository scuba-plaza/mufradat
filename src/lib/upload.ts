import fs from "node:fs";

export const MAX_IMPORT_BYTES = 3 * 1024 ** 3;

export class TooLargeError extends Error {
  constructor() {
    super("That file is larger than 3 GiB.");
  }
}

export async function saveBody(body: ReadableStream<Uint8Array>, file: string, limit: number): Promise<void> {
  const handle = await fs.promises.open(file, "w");
  const reader = body.getReader();
  let size = 0;
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      size += value.length;
      if (size > limit) throw new TooLargeError();
      await handle.writeFile(value);
    }
  } catch (error) {
    await reader.cancel().catch(() => undefined);
    throw error;
  } finally {
    await handle.close();
  }
}
