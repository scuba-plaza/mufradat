const MEGABYTE = 1024 * 1024;
const GIGABYTE = 1024 * MEGABYTE;

export const size = (bytes: number) =>
  bytes < 1024
    ? `${bytes} B`
    : bytes < MEGABYTE
      ? `${Math.round(bytes / 1024)} KB`
      : bytes < GIGABYTE
        ? `${(bytes / MEGABYTE).toFixed(1)} MB`
        : `${(bytes / GIGABYTE).toFixed(1)} GB`;

export const plural = (count: number, word: string) => (count === 1 ? word : `${word}s`);
