export {};

const rows = '[aria-label="Word list"] > li';
const pager = 'nav[aria-label="Pages"]';
const panel = '[aria-label="Next word"]';
const bar = 'form[role="search"] input';
const tabs = 'nav[aria-label="Status"]';
const sorting = '[aria-label="Sort by"]';
const typed = (query: string) => `/?q=${encodeURIComponent(query)}`;

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

  it("puts the card you worked on last at the top of the list", () => {
    cy.visit("/open?rank=120");
    cy.visit("/");
    cy.get(rows).first().find(".num").should("have.text", "120");
  });

  it("shows one page of 30 words, and moves between pages with the pager", () => {
    cy.visit("/");
    cy.get(rows).should("have.length", 30);
    cy.contains(/Page 1 of \d+/);
    cy.get(pager).first().should("not.contain", "Previous").and("not.contain", "First");
    cy.get(rows).first().invoke("text").then((first) => {
      cy.get(pager).first().contains("a", "Next").click();
      cy.location("search").should("eq", "?page=2");
      cy.contains(/Page 2 of \d+/);
      cy.get(rows).should("have.length", 30);
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

  it("opens the card of a word that has one from its row", () => {
    cy.visit("/open?rank=170");
    cy.location("pathname").then((card) => {
      cy.visit(typed("rank:170"));
      cy.wordRow(170).find("a").should("have.attr", "href").and("match", /^\/cards\/[0-9a-f-]{36}/);
      cy.wordRow(170).click();
      cy.location("pathname").should("eq", card);
    });
  });

  it("says nothing of a status for a word without a card, and opens its card from its row, creating it", () => {
    cy.visit(typed("rank:4990"));
    cy.wordRow(4990).should("not.contain", "in progress").and("not.contain", "approved").and("not.contain", "ready");
    cy.wordRow(4990).find("a").should("have.attr", "href", "/open?rank=4990&q=rank%3A4990");
    cy.wordRow(4990).click();
    cy.location("pathname").should("match", /^\/cards\/[0-9a-f-]{36}$/);
    cy.contains("Word 4990 of 5000");
  });
});

describe("continuing", () => {
  it("opens the card of the next word when you go to /next, and the same card from the Continue button", () => {
    cy.visit("/next");
    cy.location("pathname").should("match", /^\/cards\/[0-9a-f-]{36}$/);
    cy.title().should("match", /^Word \d+ · Mufradat$/);
    cy.location("pathname").then((next) => {
      cy.visit("/");
      cy.contains("a", "Continue").click();
      cy.location("pathname").should("eq", next);
    });
  });

  it("goes back to the card you edited last, not on to a word you have not started", () => {
    cy.visit("/open?rank=150");
    cy.location("pathname").then((card) => {
      cy.hydrated('[aria-label="Hint English"]');
      cy.get('[aria-label="Hint English"]').type("edited last").blur();
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
    cy.hydrated('[aria-label="Hint English"]');
    cy.contains("Word ").should("not.exist");
    cy.get('[title="Reset word"]').click();
    cy.get("dialog[open]").should("contain", "The word leaves your list; search for it to add it again.");
    cy.get("dialog[open]").contains("button", "Reset").click();
    cy.location("pathname").should("eq", "/");
    cy.get(rows).should("not.contain", "آجَرَ");
  });
});

describe("the status of a word in the list", () => {
  it("says in progress, ready and approved, drawn as three different shapes, so that colour is not the only difference", () => {
    cy.viewport(390, 844);
    cy.visit("/open?rank=4101");
    cy.finishCard(4102);
    cy.finishCard(4103);
    cy.location("pathname").then((card) => cy.request("POST", `/api${card.replace("/cards", "/notes")}/approve`, {}));
    cy.visit(typed("rank:4101-4103"));
    cy.get(rows).should("have.length", 3);
    for (const [rank, status, label] of [[4101, "progress", "in progress"], [4102, "ready", "ready"], [4103, "approved", "approved"]] as const) {
      cy.wordRow(rank).find(".status").should("have.attr", "data-status", status);
      cy.wordRow(rank).find(".status").should("contain", label);
    }
    cy.get(`${rows} .status svg`).should("be.visible").then(($marks) => {
      const shapes = Array.from($marks).map((mark) => mark.innerHTML);
      expect(new Set(shapes).size).to.equal(3);
    });
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
    cy.get(tabs).contains("a", "Not started").click();
    cy.location("search").should("eq", "?q=status%3Atodo");
    cy.get(rows).first().should("not.contain", "in progress").and("not.contain", "approved");
  });

  it("carries the query to the cards it opens, so that their arrows follow the list", () => {
    cy.visit(typed("rank:181-182"));
    cy.get(rows).first().find("a").should("have.attr", "href").and("match", /q=rank%3A181-182$/);
    cy.visit("/");
    cy.get(rows).first().find("a").should("have.attr", "href").and("not.include", "q=");
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
    cy.get(rows).should("have.length", 30);
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
