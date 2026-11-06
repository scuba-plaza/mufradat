export {};

const trigger = 'button[aria-label="Menu"]';
const popover = "header [popover]";

function openMenu() {
  cy.visit("/");
  cy.hydrated(trigger);
  cy.get(trigger).click();
}

describe("the menu", () => {
  it("opens from the ⋯ button in the header and lists Keyboard shortcuts", () => {
    cy.visit("/");
    cy.hydrated(trigger);
    cy.get(popover).should("not.be.visible");
    cy.get(trigger).click();
    cy.get(popover).should("be.visible").and("contain", "Keyboard shortcuts");
    cy.axe("header");
  });

  it("closes on Escape and gives the focus back to the button", () => {
    openMenu();
    cy.get(popover).should("be.visible");
    cy.pressEscape();
    cy.get(popover).should("not.be.visible");
    cy.focused().should("have.attr", "aria-label", "Menu");
  });

  for (const [width, height] of [[1920, 1100], [390, 844]]) {
    it(`keeps the menu inside the window and ends level with its button at ${width}px`, () => {
      cy.viewport(width, height);
      openMenu();
      cy.get(popover).should("be.visible");
      cy.get(popover).then(($popover) => {
        cy.get(trigger).then(($button) => {
          const menu = $popover[0].getBoundingClientRect();
          const button = $button[0].getBoundingClientRect();
          const doc = $popover[0].ownerDocument.documentElement;
          expect(menu.left).to.be.at.least(0);
          expect(menu.right).to.be.at.most(doc.clientWidth);
          expect(Math.abs(menu.right - button.right)).to.be.at.most(1);
          expect(doc.scrollWidth).to.be.at.most(doc.clientWidth);
        });
      });
    });
  }

  for (const [width, height] of [[1920, 1100], [390, 844]]) {
    it(`puts the button at the right end of the top bar at ${width}px`, () => {
      cy.viewport(width, height);
      cy.visit("/");
      cy.hydrated(trigger);
      cy.get(trigger).then(($button) => {
        cy.get("header .topbar-inner").then(($bar) => {
          const bar = $bar[0].getBoundingClientRect();
          const inside = bar.right - parseFloat(getComputedStyle($bar[0]).paddingRight);
          expect(Math.abs($button[0].getBoundingClientRect().right - inside)).to.be.at.most(1);
        });
      });
    });
  }

  it("gives the focus back to the menu button when the keyboard shortcuts close", () => {
    cy.visit("/");
    cy.menuItem("Keyboard shortcuts");
    cy.get("dialog[open]").contains("button", "Close").click();
    cy.get("dialog[open]").should("not.exist");
    cy.focused().should("have.attr", "aria-label", "Menu");
    cy.menuItem("Keyboard shortcuts");
    cy.get("dialog[open]");
    cy.pressEscape();
    cy.get("dialog[open]").should("not.exist");
    cy.focused().should("have.attr", "aria-label", "Menu");
  });

  it("gives the focus back to the menu button when a click on the backdrop closes it", () => {
    cy.visit("/");
    cy.menuItem("Keyboard shortcuts");
    cy.clickBackdrop();
    cy.get("dialog[open]").should("not.exist");
    cy.focused().should("have.attr", "aria-label", "Menu");
  });

  it("has Backups… and Delete unused audio… in the menu and nothing of them on the page", () => {
    openMenu();
    cy.get(popover).should("contain", "Backups…").and("contain", "Delete unused audio…");
    cy.get("main").should("not.contain", "Backups").and("not.contain", "Delete unused audio");
  });
});

const status = "header .topbar-inner > [role=\"status\"]";

function approvedCard() {
  cy.visit("/open?rank=4100");
  cy.location("pathname").then((card) =>
    cy.request({ method: "POST", url: `/api${card.replace("/cards", "/notes")}/approve`, body: {}, failOnStatusCode: false }),
  );
}

describe("export from the menu", () => {
  it("names the approved cards in its text, and is disabled when there are none", () => {
    cy.request("/api/backups").its("body.check.approved").then((approved: number) => {
      openMenu();
      if (approved === 0) {
        cy.contains(`${popover} .options-item`, "Export to Anki (nothing approved yet)").should("be.disabled");
      } else {
        cy.contains(`${popover} .options-item`, `Export ${approved} approved ${approved === 1 ? "card" : "cards"} to Anki`).should("not.be.disabled");
      }
    });
  });

  it("exports the approved cards, says where the file is and downloads it", () => {
    approvedCard();
    cy.visit("/");
    cy.intercept("GET", "/api/export?file=*").as("download");
    cy.menuItem("Export");
    cy.get(status)
      .invoke("text")
      .should("match", /^Exported \d+ cards? to .*\/exports\/arabic-deck-\d{4}-\d{2}-\d{2}\.apkg$/)
      .then((text) => {
        cy.wait("@download").its("response.statusCode").should("eq", 200);
        const name = text.slice(text.lastIndexOf("/") + 1);
        cy.readFile(`${Cypress.config("downloadsFolder")}/${name}`, null, { timeout: 15000 }).should("exist");
      });
  });

  it("says why when the export is refused and starts no download", () => {
    approvedCard();
    cy.visit("/");
    cy.intercept("POST", "/api/export", { statusCode: 400, body: { error: "The card of the word X has no audio file." } });
    cy.intercept("GET", "/api/export?file=*", cy.spy().as("download"));
    cy.menuItem("Export");
    cy.get(status).should("have.text", "The card of the word X has no audio file.");
    cy.get("@download").should("not.have.been.called");
  });

  it("says card, not cards, for one", () => {
    approvedCard();
    cy.visit("/");
    cy.intercept("POST", "/api/export", { body: { file: "arabic-deck-2026-01-01.apkg", path: "/x/exports/arabic-deck-2026-01-01.apkg", cards: 1 } });
    cy.intercept("GET", "/api/export?file=*", {
      headers: { "Content-Type": "application/octet-stream", "Content-Disposition": 'attachment; filename="arabic-deck-2026-01-01.apkg"' },
      body: "x",
    });
    cy.menuItem("Export");
    cy.get(status).should("have.text", "Exported 1 card to /x/exports/arabic-deck-2026-01-01.apkg");
  });

  it("puts the message on its own row under the button and does not scroll the page sideways at 390px", () => {
    cy.viewport(390, 844);
    approvedCard();
    cy.visit("/");
    cy.menuItem("Export");
    cy.get(status).should("contain", "Exported");
    cy.get(status).then(($message) => {
      cy.get(trigger).then(($button) => {
        expect($message[0].getBoundingClientRect().top).to.be.at.least($button[0].getBoundingClientRect().bottom - 1);
      });
    });
    cy.document().should((doc) => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
  });

  it("shows the menu on the not-found page and opens it", () => {
    cy.visit("/cards/nope", { failOnStatusCode: false });
    cy.hydrated(trigger).click();
    cy.get(popover).should("be.visible").and("contain", "Keyboard shortcuts");
  });
});

