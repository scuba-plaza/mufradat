export {};

const rows = '[aria-label="Word list"] > li';
const pager = 'nav[aria-label="Pages"]';
const panel = '[aria-label="Next word"]';
const bar = 'form[role="search"] input';
const tabs = 'nav[aria-label="Status"]';
const sorting = '[aria-label="Sort by"]';
const typed = (query: string) => `/?q=${encodeURIComponent(query)}`;
const mp3 = [0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4];

function finishCard(rank: number) {
  cy.visit(`/open?rank=${rank}`);
  cy.location("pathname").then((path) => {
    const id = path.split("/").pop();
    cy.request("PUT", `/api/notes/${id}`, { example: "كَانَ [الْكِتَابُ] جَمِيلًا جِدًّا", exampleEn: "The book was very beautiful." }).then((response) => {
      const { forms } = (response.body as { note: { forms: { id: string }[] } }).note;
      cy.window().then(async (win) => {
        for (const slot of ["word", "example", ...forms.map((form) => form.id)]) {
          await win.fetch(`/api/audio/${id}/${slot}`, { method: "PUT", body: new win.Uint8Array(mp3) });
        }
      });
    });
  });
}

describe("the words page", () => {
  it("shows the card Continue opens, large, with a Continue button, and how many cards are in each state", () => {
    cy.visit("/next");
    cy.get('[aria-label="Arabic headword"]').invoke("val").then((lemma) => {
      cy.visit("/");
      cy.get(panel).within(() => {
        cy.contains(String(lemma));
        cy.contains("a", "Continue").should("have.attr", "href", "/next");
      });
    });
    cy.contains(/\d+ approved · \d+ in progress · \d+ ready/);
  });

  it("shows one page of 100 words", () => {
    cy.visit("/");
    cy.get(rows).should("have.length", 100);
    cy.contains(/Page 1 of \d+/);
  });

  it("puts the card you worked on last at the top of the list", () => {
    cy.visit("/open?rank=120");
    cy.visit("/");
    cy.get(rows).first().find(".num").should("have.text", "120");
  });

  it("moves between pages with the pager", () => {
    cy.visit("/");
    cy.get(pager).first().should("not.contain", "Previous").and("not.contain", "First");
    cy.get(rows).first().invoke("text").then((first) => {
      cy.get(pager).first().contains("a", "Next").click();
      cy.location("search").should("eq", "?page=2");
      cy.contains(/Page 2 of \d+/);
      cy.get(rows).should("have.length", 100);
      cy.get(rows).first().invoke("text").should("not.eq", first);
      cy.get(pager).first().contains("a", "Previous").click();
      cy.contains(/Page 1 of \d+/);
      cy.get(rows).first().invoke("text").should("eq", first);
    });

    cy.get(pager).first().contains("a", "Last").click();
    cy.contains(/Page (\d+) of \1/);
    cy.get(pager).first().should("not.contain", "Next").and("not.contain", "Last");

    cy.get(pager).first().contains("a", "First").click();
    cy.contains(/Page 1 of \d+/);
  });

  it("moves a page that does not exist to the nearest one", () => {
    cy.visit("/?page=999");
    cy.contains(/Page (\d+) of \1/);
    cy.visit("/?page=0");
    cy.contains(/Page 1 of \d+/);
  });

  it("says whether a card is in progress or approved, and nothing for a word without a card", () => {
    cy.visit("/open?rank=130");
    cy.visit("/open?rank=131");
    cy.location("pathname").then((card) => cy.request("POST", `/api${card.replace("/cards", "/notes")}/approve`, {}));
    cy.visit(typed("rank:130"));
    cy.wordRow(130).should("contain", "in progress");
    cy.visit(typed("rank:131"));
    cy.wordRow(131).should("contain", "approved");
    cy.visit(typed("rank:4980"));
    cy.wordRow(4980).should("not.contain", "in progress").and("not.contain", "approved").and("not.contain", "ready");
  });

  it("says ready for a card that lacks nothing", () => {
    finishCard(141);
    cy.visit(typed("rank:141"));
    cy.wordRow(141).should("contain", "ready");
    cy.contains(panel, /\d+ ready/);
  });

  it("opens the card of a word that has one from its row", () => {
    cy.visit("/open?rank=170");
    cy.location("pathname").then((card) => {
      cy.visit(typed("rank:170"));
      cy.wordRow(170).find("a").should("have.attr", "href").and("match", /^\/cards\/[0-9a-f-]{36}/);
      cy.wordRow(170).click();
      cy.location("pathname").should("eq", card);
    });
  });

  it("opens the card of a word without one from its row, creating it", () => {
    cy.visit(typed("rank:4990"));
    cy.wordRow(4990).find("a").should("have.attr", "href", "/open?rank=4990&q=rank%3A4990");
    cy.wordRow(4990).click();
    cy.location("pathname").should("match", /^\/cards\/[0-9a-f-]{36}$/);
    cy.contains("Word 4990 of 5000");
  });
});

describe("continuing", () => {
  it("opens the card of the next word when you go to /next", () => {
    cy.visit("/next");
    cy.location("pathname").should("match", /^\/cards\/[0-9a-f-]{36}$/);
    cy.title().should("match", /^Word \d+ · Mufradat$/);
  });

  it("opens the same card from the Continue button", () => {
    cy.visit("/next");
    cy.location("pathname").then((next) => {
      cy.visit("/");
      cy.contains("a", "Continue").click();
      cy.location("pathname").should("eq", next);
    });
  });

  it("goes back to the card you edited last, not on to a word you have not started", () => {
    cy.visit("/open?rank=150");
    cy.location("pathname").then((card) => {
      cy.hydrated('[aria-label="Hint"]');
      cy.get('[aria-label="Hint"]').type("edited last").blur();
      cy.contains('[role="status"]', "Saved");
      cy.visit("/next");
      cy.location("pathname").should("eq", card);
    });
  });
});

describe("a card made outside the list", () => {
  it("is listed with a dash for its rank, and Continue offers it while it is in progress", () => {
    cy.visit(`/open?lemma=${encodeURIComponent("آنَ")}&pos=verb`);
    cy.location("pathname").then((card) => {
      cy.visit("/");
      cy.get(rows).first().should("contain", "آنَ").and("contain", "—");
      cy.visit("/next");
      cy.location("pathname").should("eq", card);
    });
  });

  it("has Reset word without a word counter, says the word leaves the list, and resets it", () => {
    cy.visit(`/open?lemma=${encodeURIComponent("آجَرَ")}&pos=verb`);
    cy.hydrated('[aria-label="Hint"]');
    cy.contains("Word ").should("not.exist");
    cy.get('[aria-label="Reset word"]').click();
    cy.get("dialog[open]").should("contain", "The word leaves your list; search for it to add it again.");
    cy.get("dialog[open]").contains("button", "Reset").click();
    cy.location("pathname").should("eq", "/");
    cy.get(rows).should("not.contain", "آجَرَ");
  });
});

describe("the search bar", () => {
  it("searches when you press Enter, keeps what you typed in the box, and counts the words it found", () => {
    cy.visit("/");
    cy.get(bar).type("كتاب{enter}");
    cy.location("search").should("eq", `?q=${encodeURIComponent("كتاب")}`);
    cy.get(bar).should("have.value", "كتاب");
    cy.contains(/^\d+ words?$/);
  });

  it("sets the Arabic typed into the bar in the Arabic font", () => {
    cy.visit("/");
    cy.get(bar).then(($input) => {
      const win = $input[0].ownerDocument.defaultView!;
      const families = (list: string) => list.split(",").map((family) => family.replace(/["']/g, "").trim());
      const [arabic] = families(win.getComputedStyle(win.document.documentElement).getPropertyValue("--font-arabic"));
      expect(arabic).to.not.equal("");
      expect(families(win.getComputedStyle($input[0]).fontFamily)).to.include(arabic);
    });
  });

  it("finds a word typed without its vowels and a word typed with a typo", () => {
    cy.visit(typed("كتاب"));
    cy.get(rows).first().should("contain", "كِتَاب").and("contain", "book");
    cy.visit(typed("writting"));
    cy.get(rows).first().should("contain", "writing");
  });

  it("filters and sorts with field terms", () => {
    cy.visit(typed("rank:1-5 sort:-rank"));
    cy.get(rows).should("have.length", 5);
    cy.get(rows).then(($rows) => {
      expect([...$rows].map((row) => row.querySelector(".num")!.textContent)).to.deep.equal(["5", "4", "3", "2", "1"]);
    });
  });

  it("says what is wrong with a term it cannot read and lists no words", () => {
    cy.visit(typed("pos:animal"));
    cy.get('[role="alert"]').should("contain", "pos: expects noun, verb");
    cy.get(rows).should("have.length", 0);
  });

  it("says so when no word matches", () => {
    cy.visit(typed("zzzzzz"));
    cy.contains("Nothing matches");
    cy.get(rows).should("have.length", 0);
  });

  it("shows the words in each state as tabs, and the tab for a state lists its words", () => {
    cy.visit("/open?rank=160");
    cy.visit("/");
    cy.get(tabs).contains("a", "In progress").invoke("text").then((label) => {
      const count = Number(label.match(/\d+/)![0]);
      expect(count).to.be.at.least(1);
      cy.get(tabs).contains("a", "In progress").click();
      cy.location("search").should("eq", "?q=status%3Aprogress+sort%3A-edited");
      cy.get(tabs).contains("a", "In progress").should("have.attr", "aria-current", "page");
      cy.contains(new RegExp(`^${count} words?$`));
      cy.get(rows).first().should("contain", "in progress");
    });
    cy.get(tabs).contains("a", /^All/).click();
    cy.location("search").should("eq", "");
    cy.get(tabs).contains("a", /^All/).should("have.attr", "aria-current", "page");
  });

  it("carries the query to the cards it opens, so that their arrows follow the list", () => {
    cy.visit(typed("rank:181-182"));
    cy.get(rows).first().find("a").should("have.attr", "href").and("match", /q=rank%3A181-182$/);
    cy.visit("/");
    cy.get(rows).first().find("a").should("have.attr", "href").and("not.include", "q=");
  });

  it("lists the words not started under their tab", () => {
    cy.visit("/");
    cy.get(tabs).contains("a", "Not started").click();
    cy.location("search").should("eq", "?q=status%3Atodo");
    cy.get(rows).first().should("not.contain", "in progress").and("not.contain", "approved");
  });

  it("sorts by a column when you click its heading, the other way on the next click, and back to the default on the third", () => {
    const english = () =>
      cy.get(rows).then(($rows) => [...$rows].map((row) => row.querySelector("a")!.children[2].textContent!.toLowerCase()));
    cy.visit("/");
    cy.get(sorting).contains("a", "English").click();
    cy.location("search").should("eq", "?q=sort%3Aenglish");
    english().then((names) => expect(names).to.deep.equal([...names].sort()));
    cy.get(sorting).contains("a", "English").should("have.attr", "aria-current", "true").click();
    cy.location("search").should("eq", "?q=sort%3A-english");
    english().then((names) => expect(names).to.deep.equal([...names].sort().reverse()));
    cy.get(sorting).contains("a", "English").click();
    cy.location("search").should("eq", "");
  });

  it("changes only the sort of what you typed when you click a heading", () => {
    cy.visit(typed('pos:noun "book" sort:rank'));
    cy.get(sorting).contains("a", "Arabic").click();
    cy.location("search").should((search) => expect(new URLSearchParams(search).get("q")).to.equal('pos:noun "book" sort:arabic'));
  });

  it("keeps the query when you move between pages", () => {
    cy.visit(typed("pos:noun"));
    cy.get(pager).first().contains("a", "Next").click();
    cy.location("search").should("eq", "?q=pos%3Anoun&page=2");
    cy.get(rows).should("have.length", 100);
    cy.get(pager).first().contains("a", "First").click();
    cy.location("search").should("eq", "?q=pos%3Anoun");
  });

  it("lists Wiktionary entries that have no card among the words, each opening its own card", () => {
    const search = typed("على rank:none");
    cy.visit(search);
    cy.get(rows).should("have.length.at.least", 3);
    [0, 1, 2].forEach((i) => {
      cy.visit(search);
      cy.get(rows).eq(i).find('[lang="ar"]').invoke("text").then((lemma) => {
        expect(lemma.replace(/[\u064B-\u0652]/g, "").replace("ى", "ي"), "the letters of the entry").to.equal("علي");
        cy.get(rows).eq(i).click();
        cy.get('[aria-label="Arabic headword"]').should("have.value", lemma);
      });
    });
  });
});
