import fs from "node:fs";
import { crc32 } from "node:zlib";
import { MAX_AUDIO_BYTES } from "./audio";
import { DOS_DATE, DOS_TIME } from "./zip";

export interface BackupEntry {
  name: string;
  file: string;
}

export interface BackupZip {
  names: string[];
  read(name: string): Buffer;
  copyTo(name: string, dest: string): void;
  close(): void;
}

const LOCAL = 0x04034b50;
const CENTRAL = 0x02014b50;
const END = 0x06054b50;
const DESCRIPTOR = 0x08074b50;
const FLAGS = 0x0808;
const LIMIT = 0xffffffff;

function localHeader(nameBytes: Buffer): Buffer {
  const header = Buffer.alloc(30);
  header.writeUInt32LE(LOCAL, 0);
  header.writeUInt16LE(20, 4);
  header.writeUInt16LE(FLAGS, 6);
  header.writeUInt16LE(DOS_TIME, 10);
  header.writeUInt16LE(DOS_DATE, 12);
  header.writeUInt16LE(nameBytes.length, 26);
  return header;
}

function descriptor(crc: number, size: number): Buffer {
  const record = Buffer.alloc(16);
  record.writeUInt32LE(DESCRIPTOR, 0);
  record.writeUInt32LE(crc, 4);
  record.writeUInt32LE(size, 8);
  record.writeUInt32LE(size, 12);
  return record;
}

function centralHeader(nameBytes: Buffer, crc: number, size: number, offset: number): Buffer {
  const header = Buffer.alloc(46);
  header.writeUInt32LE(CENTRAL, 0);
  header.writeUInt16LE(20, 4);
  header.writeUInt16LE(20, 6);
  header.writeUInt16LE(FLAGS, 8);
  header.writeUInt16LE(DOS_TIME, 12);
  header.writeUInt16LE(DOS_DATE, 14);
  header.writeUInt32LE(crc, 16);
  header.writeUInt32LE(size, 20);
  header.writeUInt32LE(size, 24);
  header.writeUInt16LE(nameBytes.length, 28);
  header.writeUInt32LE(offset, 42);
  return header;
}

async function* chunks(entries: BackupEntry[]): AsyncGenerator<Uint8Array> {
  if (entries.length >= 0xffff) throw new Error("The backup has too many files for one zip.");
  const central: Buffer[] = [];
  let offset = 0;
  for (const { name, file } of entries) {
    const nameBytes = Buffer.from(name, "utf8");
    yield localHeader(nameBytes);
    yield nameBytes;
    let crc = 0;
    let size = 0;
    for await (const chunk of fs.createReadStream(file)) {
      const bytes = chunk as Buffer;
      crc = crc32(bytes, crc);
      size += bytes.length;
      yield bytes;
    }
    if (size > LIMIT) throw new Error(`${name} is too large for a backup file.`);
    yield descriptor(crc, size);
    central.push(centralHeader(nameBytes, crc, size, offset), nameBytes);
    offset += 30 + nameBytes.length + size + 16;
    if (offset > LIMIT) throw new Error("The backup is too large for one file.");
  }
  const directory = Buffer.concat(central);
  yield directory;
  const end = Buffer.alloc(22);
  end.writeUInt32LE(END, 0);
  end.writeUInt16LE(entries.length, 8);
  end.writeUInt16LE(entries.length, 10);
  end.writeUInt32LE(directory.length, 12);
  end.writeUInt32LE(offset, 16);
  yield end;
}

export function backupZipStream(entries: BackupEntry[]): ReadableStream<Uint8Array> {
  const source = chunks(entries);
  return new ReadableStream<Uint8Array>({
    async pull(controller) {
      const next = await source.next();
      if (next.done) controller.close();
      else controller.enqueue(next.value);
    },
    async cancel() {
      await source.return(undefined);
    },
  });
}

interface Found {
  crc: number;
  size: number;
  offset: number;
}

export const NOT_A_BACKUP = "This is not a Mufradat backup.";
const ENTRY_NAME = /^(manifest\.json|vocabulary\.db|audio\/([0-9a-f]{2})\/\2[0-9a-f]{14}\.mp3)$/;
const COPY_CHUNK = 1024 * 1024;

const refuse = (): never => {
  throw new Error(NOT_A_BACKUP);
};

function parse(fd: number): BackupZip {
  const total = fs.fstatSync(fd).size;
  const readAt = (length: number, position: number): Buffer => {
    if (position < 0 || position + length > total) return refuse();
    const buffer = Buffer.alloc(length);
    if (fs.readSync(fd, buffer, 0, length, position) !== length) return refuse();
    return buffer;
  };
  const end = readAt(22, total - 22);
  if (end.readUInt32LE(0) !== END || end.readUInt16LE(4) !== 0 || end.readUInt16LE(6) !== 0 || end.readUInt16LE(20) !== 0) refuse();
  const count = end.readUInt16LE(10);
  if (end.readUInt16LE(8) !== count || count === 0xffff) refuse();
  const directorySize = end.readUInt32LE(12);
  const directoryAt = end.readUInt32LE(16);
  if (directorySize === LIMIT || directoryAt === LIMIT || directoryAt + directorySize !== total - 22) refuse();
  const directory = readAt(directorySize, directoryAt);
  const found = new Map<string, Found>();
  let at = 0;
  for (let i = 0; i < count; i++) {
    if (at + 46 > directory.length || directory.readUInt32LE(at) !== CENTRAL) refuse();
    const flags = directory.readUInt16LE(at + 8);
    const size = directory.readUInt32LE(at + 24);
    const nameLength = directory.readUInt16LE(at + 28);
    const offset = directory.readUInt32LE(at + 42);
    const plain = (flags & ~FLAGS) === 0 && directory.readUInt16LE(at + 10) === 0 && directory.readUInt16LE(at + 34) === 0;
    if (!plain || size !== directory.readUInt32LE(at + 20) || size === LIMIT || offset === LIMIT) refuse();
    const name = directory.subarray(at + 46, at + 46 + nameLength).toString("utf8");
    if (!ENTRY_NAME.test(name) || found.has(name)) refuse();
    found.set(name, { crc: directory.readUInt32LE(at + 16), size, offset });
    at += 46 + nameLength + directory.readUInt16LE(at + 30) + directory.readUInt16LE(at + 32);
  }
  if (at !== directory.length) refuse();

  const locate = (name: string): { entry: Found; start: number } => {
    const entry = found.get(name);
    if (!entry) return refuse();
    const local = readAt(30, entry.offset);
    const nameLength = local.readUInt16LE(26);
    if (local.readUInt32LE(0) !== LOCAL || readAt(nameLength, entry.offset + 30).toString("utf8") !== name) return refuse();
    const start = entry.offset + 30 + nameLength + local.readUInt16LE(28);
    if (start + entry.size > directoryAt) return refuse();
    return { entry, start };
  };

  return {
    names: [...found.keys()],
    read(name) {
      const { entry, start } = locate(name);
      if (name !== "vocabulary.db" && entry.size > MAX_AUDIO_BYTES) refuse();
      const bytes = readAt(entry.size, start);
      if (crc32(bytes) !== entry.crc) refuse();
      return bytes;
    },
    copyTo(name, dest) {
      const { entry, start } = locate(name);
      const out = fs.openSync(dest, "w");
      let crc = 0;
      let sound = false;
      try {
        for (let done = 0; done < entry.size; done += COPY_CHUNK) {
          const chunk = readAt(Math.min(COPY_CHUNK, entry.size - done), start + done);
          crc = crc32(chunk, crc);
          fs.writeSync(out, chunk);
        }
        sound = crc === entry.crc;
      } finally {
        fs.closeSync(out);
        if (!sound) fs.rmSync(dest, { force: true });
      }
      if (!sound) refuse();
    },
    close: () => fs.closeSync(fd),
  };
}

export function openBackupZip(file: string): BackupZip {
  const fd = fs.openSync(file, "r");
  try {
    return parse(fd);
  } catch (error) {
    fs.closeSync(fd);
    throw error;
  }
}
