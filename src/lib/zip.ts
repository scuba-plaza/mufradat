import { crc32 } from "node:zlib";

// 2000-01-01 00:00:00 in DOS format, so the same files always zip to the same bytes.
export const DOS_TIME = 0;
export const DOS_DATE = ((2000 - 1980) << 9) | (1 << 5) | 1;

export const LOCAL_SIGNATURE = 0x04034b50;
export const CENTRAL_SIGNATURE = 0x02014b50;
export const END_SIGNATURE = 0x06054b50;
const UTF8_NAMES = 0x0800;

interface ZipEntry {
  name: string;
  data: Uint8Array;
}

/** The central directory record of a stored entry. */
export function centralHeader(nameBytes: Buffer, flags: number, crc: number, size: number, offset: number): Buffer {
  const header = Buffer.alloc(46);
  header.writeUInt32LE(CENTRAL_SIGNATURE, 0);
  header.writeUInt16LE(20, 4);
  header.writeUInt16LE(20, 6);
  header.writeUInt16LE(flags, 8);
  header.writeUInt16LE(DOS_TIME, 12);
  header.writeUInt16LE(DOS_DATE, 14);
  header.writeUInt32LE(crc, 16);
  header.writeUInt32LE(size, 20);
  header.writeUInt32LE(size, 24);
  header.writeUInt16LE(nameBytes.length, 28);
  header.writeUInt32LE(offset, 42);
  return header;
}

export function endOfDirectory(count: number, directorySize: number, directoryAt: number): Buffer {
  const end = Buffer.alloc(22);
  end.writeUInt32LE(END_SIGNATURE, 0);
  end.writeUInt16LE(count, 8);
  end.writeUInt16LE(count, 10);
  end.writeUInt32LE(directorySize, 12);
  end.writeUInt32LE(directoryAt, 16);
  return end;
}

/** A zip with every entry stored uncompressed, in the order given. */
export function zipStored(entries: ZipEntry[]): Buffer<ArrayBuffer> {
  const parts: Buffer[] = [];
  const central: Buffer[] = [];
  let offset = 0;
  for (const { name, data } of entries) {
    const nameBytes = Buffer.from(name, "utf8");
    const crc = crc32(data);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(LOCAL_SIGNATURE, 0);
    local.writeUInt16LE(20, 4);
    local.writeUInt16LE(UTF8_NAMES, 6);
    local.writeUInt16LE(DOS_TIME, 10);
    local.writeUInt16LE(DOS_DATE, 12);
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(data.length, 18);
    local.writeUInt32LE(data.length, 22);
    local.writeUInt16LE(nameBytes.length, 26);
    parts.push(local, nameBytes, Buffer.from(data));
    central.push(centralHeader(nameBytes, UTF8_NAMES, crc, data.length, offset), nameBytes);
    offset += 30 + nameBytes.length + data.length;
  }
  const directory = Buffer.concat(central);
  return Buffer.concat([...parts, directory, endOfDirectory(entries.length, directory.length, offset)]);
}
