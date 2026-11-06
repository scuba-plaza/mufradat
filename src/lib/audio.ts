import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";

const MAX_AUDIO_BYTES = 10 * 1024 * 1024;

/** First 16 hex characters of the SHA-256 of the audio bytes. */
export function audioKey(bytes: Uint8Array): string {
  return createHash("sha256").update(bytes).digest("hex").slice(0, 16);
}

export function audioPath(root: string, key: string): string {
  return path.join(root, "audio", key.slice(0, 2), `${key}.mp3`);
}

export function audioFileName(key: string): string {
  return `${key}.mp3`;
}

export interface AudioFile {
  key: string;
  path: string;
  bytes: number;
}

const STORED_NAME = /^([0-9a-f]{2})[0-9a-f]{14}\.mp3$/;

export function audioFiles(root: string): AudioFile[] {
  const base = path.join(root, "audio");
  if (!fs.existsSync(base)) return [];
  return fs.readdirSync(base, { withFileTypes: true }).flatMap((folder) => {
    if (!folder.isDirectory()) return [];
    return fs.readdirSync(path.join(base, folder.name), { withFileTypes: true }).flatMap((entry) => {
      const match = STORED_NAME.exec(entry.name);
      if (!entry.isFile() || !match || match[1] !== folder.name) return [];
      const file = path.join(base, folder.name, entry.name);
      return [{ key: entry.name.slice(0, -".mp3".length), path: file, bytes: fs.statSync(file).size }];
    });
  });
}

/** Returns the reason the bytes are not usable MP3 audio, or null. */
export function mp3Problem(bytes: Uint8Array): string | null {
  if (bytes.length === 0) return "The audio is empty.";
  if (bytes.length > MAX_AUDIO_BYTES) return "The audio is larger than 10 MB.";
  const id3 = bytes[0] === 0x49 && bytes[1] === 0x44 && bytes[2] === 0x33;
  const frame = bytes[0] === 0xff && (bytes[1] & 0xe0) === 0xe0;
  return id3 || frame ? null : "The audio is not an MP3 file.";
}
