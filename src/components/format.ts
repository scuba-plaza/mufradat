const MEGABYTE = 1024 * 1024;
const GIGABYTE = 1024 * MEGABYTE;

export function size(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < MEGABYTE) return `${Math.round(bytes / 1024)} KB`;
  if (bytes < GIGABYTE) return `${(bytes / MEGABYTE).toFixed(1)} MB`;
  return `${(bytes / GIGABYTE).toFixed(1)} GB`;
}

export const plural = (count: number, word: string) => (count === 1 ? word : `${word}s`);
