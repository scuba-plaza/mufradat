import { describe, expect, it } from "vitest";
import { paginate, requestedPage } from "./paging";

describe("paginating the frequency list", () => {
  it("gives the first of 50 pages for 5000 words in pages of 100", () => {
    expect(paginate(5000, 1, 100)).toEqual({ page: 1, pages: 50, start: 0, end: 100, prev: null, next: 2 });
  });

  it("gives the last page no next page and a previous one", () => {
    expect(paginate(5000, 50, 100)).toEqual({ page: 50, pages: 50, start: 4900, end: 5000, prev: 49, next: null });
  });

  it("ends a short last page at the last item", () => {
    expect(paginate(250, 3, 100)).toEqual({ page: 3, pages: 3, start: 200, end: 250, prev: 2, next: null });
  });

  it("moves a page number that is out of range to the nearest page", () => {
    expect(paginate(5000, 51, 100).page).toBe(50);
    expect(paginate(5000, 9999, 100).page).toBe(50);
    expect(paginate(5000, 0, 100).page).toBe(1);
    expect(paginate(5000, -3, 100).page).toBe(1);
  });

  it("treats a page number that is not a whole number sensibly", () => {
    expect(paginate(5000, Number.NaN, 100).page).toBe(1);
    expect(paginate(5000, 2.7, 100).page).toBe(2);
  });

  it("still has one, empty, page when there is nothing to list", () => {
    expect(paginate(0, 1, 100)).toEqual({ page: 1, pages: 1, start: 0, end: 0, prev: null, next: null });
  });
});

describe("choosing the page to show", () => {
  it("shows the page asked for", () => {
    expect(requestedPage({ page: "3" })).toBe(3);
  });

  it("opens on the first page when nothing is asked for", () => {
    expect(requestedPage({})).toBe(1);
  });

  it("falls back to the first page when what was asked for is not a number", () => {
    expect(requestedPage({ page: "abc" })).toBe(1);
    expect(requestedPage({ page: "" })).toBe(1);
  });
});
