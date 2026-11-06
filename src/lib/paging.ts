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

export const firstParam = (value: string | string[] | undefined): string => (Array.isArray(value) ? value[0] : value) ?? "";

/** `?page=N`; a page that is missing or unreadable is the first. */
export function requestedPage(query: { page?: string | string[] }): number {
  const text = firstParam(query.page);
  const page = text.trim() === "" ? Number.NaN : Number(text);
  return Number.isFinite(page) ? page : 1;
}

export function listPath(q: string, page = 1): string {
  const params = new URLSearchParams();
  if (q !== "") params.set("q", q);
  if (page > 1) params.set("page", String(page));
  const query = params.toString();
  return query === "" ? "/" : `/?${query}`;
}
