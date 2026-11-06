export {};

const rows = '[aria-label="Word list"] > li';
const pager = 'nav[aria-label="Pages"]';
const panel = '[aria-label="Next word"]';
const mp3 = [0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4];

/** The row of the word with this rank, wherever it stands in the list. */
const rowOf = (rank: number) => cy.get(rows).filter((_, row) => row.querySelector(".num")?.textContent === String(rank));

/** Opens the card of a word and gives it everything a card needs, so that nothing is missing. */
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
    cy.visit("/");
    rowOf(130).should("contain", "in progress");
    cy.visit("/?page=999");
    rowOf(131).should("contain", "approved");
    rowOf(4990).should("not.contain", "in progress").and("not.contain", "approved").and("not.contain", "ready");
  });

  it("says ready for a card that lacks nothing", () => {
    finishCard(141);
    cy.visit("/");
    rowOf(141).should("contain", "ready");
    cy.contains(panel, /\d+ ready/);
  });

  it("opens the card of a word that has one from its row", () => {
    cy.visit("/");
    cy.get(rows).first().click();
    cy.location("pathname").should("match", /^\/cards\/[0-9a-f-]{36}$/);
  });

  it("opens the card of a word without one from its row, creating it", () => {
    cy.visit("/?page=999");
    rowOf(4990).click();
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
});
