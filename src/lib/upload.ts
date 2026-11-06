import { once } from "node:events";
import fs from "node:fs";

export const MAX_IMPORT_BYTES = 3 * 1024 ** 3;

export class TooLargeError extends Error {
  constructor() {
    super("That file is larger than 3 GiB.");
  }
}

export async function saveBody(body: ReadableStream<Uint8Array>, file: string, limit: number): Promise<void> {
  const out = fs.createWriteStream(file);
  const reader = body.getReader();
  let size = 0;
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      size += value.length;
      if (size > limit) throw new TooLargeError();
      if (!out.write(value)) await once(out, "drain");
    }
    await new Promise<void>((resolve, reject) => out.end((error?: Error | null) => (error ? reject(error) : resolve())));
  } catch (error) {
    out.destroy();
    await reader.cancel().catch(() => undefined);
    throw error;
  }
}
