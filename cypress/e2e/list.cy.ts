export {};

const rows = '[aria-label="Frequency list"] > li';
const pager = 'nav[aria-label="Pages"]';

describe("the words page", () => {
  it("shows the next word large, with a Continue button, and how many words are approved", () => {
    cy.visit("/");
    cy.get('[aria-label="Next word"]').within(() => {
      cy.contains("فِي");
      cy.contains("in, within");
      cy.contains("a", "Continue").should("have.attr", "href", "/next");
    });
    cy.contains(/\d+ of 5000 approved/);
  });

  it("shows one page of 100 words, starting at rank 1 in a fresh workspace", () => {
    cy.visit("/");
    cy.get(rows).should("have.length", 100);
    cy.get(rows).first().should("contain", "فِي").and("contain", "in, within");
    cy.contains("Page 1 of 50");
  });

  it("moves between pages with the pager", () => {
    cy.visit("/");
    cy.get(pager).first().should("not.contain", "Previous").and("not.contain", "First");
    cy.get(pager).first().contains("a", "Next").click();
    cy.location("search").should("eq", "?page=2");
    cy.contains("Page 2 of 50");
    cy.get(rows).should("have.length", 100);
    cy.get(rows).first().should("contain", "101");

    cy.get(pager).first().contains("a", "Previous").click();
    cy.contains("Page 1 of 50");

    cy.get(pager).first().contains("a", "Last").click();
    cy.contains("Page 50 of 50");
    cy.get(rows).last().should("contain", "5000");
    cy.get(pager).first().should("not.contain", "Next").and("not.contain", "Last");

    cy.get(pager).first().contains("a", "First").click();
    cy.contains("Page 1 of 50");
  });

  it("jumps to the page of a rank and marks its row", () => {
    cy.visit("/");
    cy.get('input[aria-label="Go to rank"]').first().type("2500{enter}");
    cy.location("search").should("eq", "?rank=2500");
    cy.contains("Page 25 of 50");
    cy.get(`${rows}[aria-current="location"]`).should("have.length", 1).and("contain", "2500");
  });

  it("moves a page that does not exist to the nearest one", () => {
    cy.visit("/?page=999");
    cy.contains("Page 50 of 50");
    cy.visit("/?page=0");
    cy.contains("Page 1 of 50");
  });

  it("says whether a word is in progress or approved, and nothing for a word without a card", () => {
    cy.visit("/");
    cy.get(rows).eq(34).should("not.contain", "in progress").and("not.contain", "approved");
    cy.visit("/open?rank=35");
    cy.visit("/open?rank=36");
    cy.location("pathname").then((card) => cy.request("POST", `/api${card.replace("/cards", "/notes")}/approve`, {}));
    cy.visit("/");
    cy.get(rows).eq(34).should("contain", "35").and("contain", "in progress");
    cy.get(rows).eq(35).should("contain", "36").and("contain", "approved");
    cy.get(rows).eq(36).should("contain", "37").and("not.contain", "in progress").and("not.contain", "approved");
  });

  it("opens the card of a word from its row", () => {
    cy.visit("/");
    cy.get(rows).eq(2).click();
    cy.location("pathname").should("match", /^\/cards\/[0-9a-f-]{36}$/);
    cy.contains("Word 3 of 5000");
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
});
