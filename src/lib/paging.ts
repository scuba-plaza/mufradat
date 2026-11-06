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

/** `?page=N`; a page that is missing or unreadable is the first. */
export function requestedPage(query: { page?: string }): number {
  const page = query.page === undefined || query.page.trim() === "" ? Number.NaN : Number(query.page);
  return Number.isFinite(page) ? page : 1;
}
