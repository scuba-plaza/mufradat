import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { backupZipStream, openBackupZip, type BackupEntry } from "./backup-zip";
import { zipStored } from "./zip";

const dirs: string[] = [];

afterEach(() => {
  for (const dir of dirs.splice(0)) fs.rmSync(dir, { recursive: true, force: true });
});

function scratch() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-zip-"));
  dirs.push(dir);
  return dir;
}

async function drain(stream: ReadableStream<Uint8Array>, file: string) {
  const reader = stream.getReader();
  const parts: Uint8Array[] = [];
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    parts.push(value);
  }
  fs.writeFileSync(file, Buffer.concat(parts));
}

const CLIP = "audio/ab/abcdef0123456789.mp3";
const manifest = Buffer.from('{"cards":1}');
const database = Buffer.alloc(200_000, 7);
const clip = Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]);

async function written(contents: [string, Buffer][]) {
  const dir = scratch();
  const entries: BackupEntry[] = contents.map(([name, bytes], i) => {
    const file = path.join(dir, `source-${i}`);
    fs.writeFileSync(file, bytes);
    return { name, file };
  });
  const zip = path.join(dir, "backup.zip");
  await drain(backupZipStream(entries), zip);
  return { dir, zip };
}

describe("reading back what the writer wrote", () => {
  it("gives the same names and the same bytes", async () => {
    const { dir, zip } = await written([["manifest.json", manifest], ["vocabulary.db", database], [CLIP, clip]]);
    const backup = openBackupZip(zip);
    expect([...backup.names].sort()).toEqual([CLIP, "manifest.json", "vocabulary.db"]);
    expect(backup.read("manifest.json")).toEqual(manifest);
    expect(backup.read(CLIP)).toEqual(clip);
    const out = path.join(dir, "out.db");
    backup.copyTo("vocabulary.db", out);
    expect(fs.readFileSync(out)).toEqual(database);
    backup.close();
  });
});

describe("refusing what is not a backup", () => {
  const refused = "This is not a Mufradat backup.";
  const stored = (contents: [string, Buffer][]) => zipStored(contents.map(([name, data]) => ({ name, data })));
  const valid = (): [string, Buffer][] => [["manifest.json", manifest], ["vocabulary.db", database], [CLIP, clip]];
  const patchCentral = (zip: Buffer, offset: number, value: number) => {
    zip.writeUInt16LE(value, zip.indexOf(Buffer.from([0x50, 0x4b, 0x01, 0x02])) + offset);
    return zip;
  };
  const flipped = () => {
    const zip = stored(valid());
    zip[zip.indexOf(manifest)] ^= 0xff;
    return zip;
  };

  const cases: [string, () => Buffer][] = [
    ["a compressed entry", () => patchCentral(stored(valid()), 10, 8)],
    ["an encrypted entry", () => patchCentral(stored(valid()), 8, 0x0801)],
    ["a duplicate name", () => stored([...valid(), ["manifest.json", manifest]])],
    ["a name that climbs out of the folder", () => stored([...valid(), ["../x", manifest]])],
    ["an unexpected entry", () => stored([...valid(), ["readme.txt", manifest]])],
    ["a clip whose folder is not the start of its key", () => stored([...valid(), ["audio/cd/abcdef0123456789.mp3", clip]])],
    ["a changed data byte", flipped],
    ["a file that stops early", () => stored(valid()).subarray(0, stored(valid()).length - 10)],
  ];

  it.each(cases)("refuses %s", (_name, build) => {
    const file = path.join(scratch(), "bad.zip");
    fs.writeFileSync(file, build());
    expect(() => {
      const backup = openBackupZip(file);
      for (const name of backup.names) backup.read(name);
    }).toThrow(refused);
  });

  it("refuses to read a manifest or clip larger than a clip may be", () => {
    const file = path.join(scratch(), "big.zip");
    fs.writeFileSync(file, stored([["manifest.json", Buffer.alloc(10 * 1024 * 1024 + 1)]]));
    const backup = openBackupZip(file);
    expect(() => backup.read("manifest.json")).toThrow(refused);
  });

  it("still reads a zip written by the plain stored writer", () => {
    const file = path.join(scratch(), "plain.zip");
    fs.writeFileSync(file, stored(valid()));
    const backup = openBackupZip(file);
    expect([...backup.names].sort()).toEqual([CLIP, "manifest.json", "vocabulary.db"]);
    expect(backup.read("manifest.json")).toEqual(manifest);
  });
});
