export {};

const kitab = encodeURIComponent("كتاب");

describe("search", () => {
  it("opens from the header", () => {
    cy.visit("/");
    cy.contains("header a", "Search").click();
    cy.location("pathname").should("eq", "/search");
  });

  it("asks for focus when it opens, and keeps the query in its box and in the page title", () => {
    cy.visit("/search");
    // The page asks for focus with autofocus, which the browser applies to a page of its own but not to the frame Cypress shows it in.
    cy.get('input[aria-label="Search"]').should("have.attr", "autofocus");
    cy.get('input[aria-label="Search"]').type("كتاب{enter}");
    cy.location("pathname").should("eq", "/search");
    cy.get('input[aria-label="Search"]').should("have.value", "كتاب");
    cy.title().should("eq", "Search: كتاب · Mufradat");
  });

  it("sets the Arabic typed into the search box in the Arabic font", () => {
    cy.visit("/search");
    cy.get('input[aria-label="Search"]').then(($input) => {
      const win = $input[0].ownerDocument.defaultView!;
      const families = (list: string) => list.split(",").map((family) => family.replace(/["']/g, "").trim());
      const [arabic] = families(win.getComputedStyle(win.document.documentElement).getPropertyValue("--font-arabic"));
      expect(arabic).to.not.equal("");
      expect(families(win.getComputedStyle($input[0]).fontFamily)).to.include(arabic);
    });
  });

  it("says how many results each group has and says so when there are none", () => {
    cy.visit("/search?q=zzz");
    cy.contains("h2", "Your cards (0)");
    cy.contains("h2", "Wiktionary (0)");
    cy.contains("Nothing matches");
    cy.visit(`/search?q=${kitab}`);
    cy.contains("h2", "Wiktionary (2)");
  });

  it("opens a Wiktionary entry as a card, which the next search then finds", () => {
    cy.visit(`/search?q=${kitab}`);
    cy.contains("h2", "Wiktionary").parent().find("a").first().click();
    cy.location("pathname").should("match", /^\/cards\/[0-9a-f-]{36}$/);
    cy.location("pathname").then((card) => {
      cy.visit(`/search?q=${kitab}`);
      cy.contains("h2", "Your cards (1)").parent().find("a").first().should("have.attr", "href", card);
    });
  });
});
