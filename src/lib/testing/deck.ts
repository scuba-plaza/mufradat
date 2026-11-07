import Database from "better-sqlite3";
import { expect } from "vitest";

export function unzip(buf: Buffer): Map<string, Buffer> {
  const out = new Map<string, Buffer>();
  let p = 0;
  while (buf.readUInt32LE(p) === 0x04034b50) {
    const method = buf.readUInt16LE(p + 8);
    const size = buf.readUInt32LE(p + 18);
    const nameLen = buf.readUInt16LE(p + 26);
    const extraLen = buf.readUInt16LE(p + 28);
    const name = buf.subarray(p + 30, p + 30 + nameLen).toString();
    expect(method).toBe(0);
    const start = p + 30 + nameLen + extraLen;
    out.set(name, buf.subarray(start, start + size));
    p = start + size;
  }
  return out;
}

export function deckGuids(apkg: Buffer): string[] {
  const db = new Database(unzip(apkg).get("collection.anki2")!);
  try {
    return (db.prepare("SELECT guid FROM notes ORDER BY guid").all() as { guid: string }[]).map((r) => r.guid);
  } finally {
    db.close();
  }
}
