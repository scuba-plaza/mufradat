export const MAX_TAG_LENGTH = 40;
export const MAX_TAGS = 20;

export function parseTags(input: unknown): { tags: string[] } | { error: string } {
  if (!Array.isArray(input) || !input.every((entry) => typeof entry === "string")) {
    return { error: "tags must be a list of text." };
  }
  const tags = new Set<string>();
  for (const entry of input as string[]) {
    const tag = entry.trim().toLowerCase().replace(/[\s\u0085]+/g, "_");
    if (tag === "") continue;
    if (/[,"*]/.test(tag)) return { error: "A tag cannot contain commas, quotes or *." };
    if (tag.length > MAX_TAG_LENGTH) return { error: `A tag is at most ${MAX_TAG_LENGTH} characters.` };
    tags.add(tag);
  }
  if (tags.size > MAX_TAGS) return { error: `A card has at most ${MAX_TAGS} tags.` };
  return { tags: [...tags].sort() };
}
