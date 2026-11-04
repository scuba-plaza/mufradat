import { describe, expect, it } from "vitest";
import { pageForRank, paginate, requestedPage } from "./paging";

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

describe("finding the page of a rank", () => {
  it("counts ranks 1 to 100 as page 1 and 101 as page 2", () => {
    expect(pageForRank(1, 100)).toBe(1);
    expect(pageForRank(100, 100)).toBe(1);
    expect(pageForRank(101, 100)).toBe(2);
  });

  it("puts rank 2500 on page 25 and rank 2501 on page 26", () => {
    expect(pageForRank(2500, 100)).toBe(25);
    expect(pageForRank(2501, 100)).toBe(26);
  });
});

describe("choosing the page to show", () => {
  it("shows the page asked for, even when a rank is given too", () => {
    expect(requestedPage({ page: "3", rank: "900" }, 5, 100)).toBe(3);
  });

  it("shows the page of a rank when only a rank is given", () => {
    expect(requestedPage({ rank: "2500" }, 5, 100)).toBe(25);
  });

  it("opens on the page of the next word when nothing is asked for", () => {
    expect(requestedPage({}, 101, 100)).toBe(2);
    expect(requestedPage({}, 100, 100)).toBe(1);
  });

  it("opens on the first page when nothing is asked for and no word is left", () => {
    expect(requestedPage({}, null, 100)).toBe(1);
  });

  it("falls back to the first page when what was asked for is not a number", () => {
    expect(requestedPage({ page: "abc" }, 500, 100)).toBe(1);
    expect(requestedPage({ rank: "x" }, 500, 100)).toBe(1);
    expect(requestedPage({ page: "" }, 500, 100)).toBe(1);
  });
});
