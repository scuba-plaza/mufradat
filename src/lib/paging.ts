export interface Paging {
  page: number;
  pages: number;
  /** Slice bounds of the page: `items.slice(start, end)`. */
  start: number;
  end: number;
  prev: number | null;
  next: number | null;
}

/** The page asked for, moved into range; a page that is not a number is the first. */
export function paginate(total: number, requested: number, size: number): Paging {
  const pages = Math.max(1, Math.ceil(total / size));
  const page = Number.isFinite(requested) ? Math.min(Math.max(Math.floor(requested), 1), pages) : 1;
  return {
    page,
    pages,
    start: Math.min((page - 1) * size, total),
    end: Math.min(page * size, total),
    prev: page > 1 ? page - 1 : null,
    next: page < pages ? page + 1 : null,
  };
}

export const pageForRank = (rank: number, size: number): number => Math.ceil(rank / size);

/** `?page=N` wins, then `?rank=N`, then the page of the next word to do; anything unreadable is page 1. */
export function requestedPage(query: { page?: string; rank?: string }, nextRank: number | null, size: number): number {
  const number = (text: string) => (text.trim() === "" ? Number.NaN : Number(text));
  const chosen =
    query.page !== undefined ? number(query.page) : query.rank !== undefined ? pageForRank(number(query.rank), size) : pageForRank(nextRank ?? 1, size);
  return Number.isFinite(chosen) ? chosen : 1;
}
