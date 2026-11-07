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
          const inside = bar.right - parseFloat(getComputedStyle($bar[0]).paddingRight) - parseFloat(getComputedStyle($button[0]).marginRight);
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

  it("has Backups and Prune audio in the menu and nothing of them on the page", () => {
    openMenu();
    cy.get(popover).should("contain", "Backups").and("contain", "Prune audio");
    cy.get("main").should("not.contain", "Backups").and("not.contain", "Prune audio");
  });

  it("lists Export, Backups, Prune audio and Keyboard shortcuts in that order", () => {
    openMenu();
    cy.get(`${popover} .options-item`).then(($items) => {
      expect([...$items].map((item) => item.textContent)).to.deep.equal(["Export", "Backups", "Prune audio", "Keyboard shortcuts"]);
    });
  });

  for (const [width, height] of [[1280, 800], [390, 844]]) {
    it(`puts a small margin around the button at ${width}px`, () => {
      cy.viewport(width, height);
      cy.visit("/");
      cy.hydrated(trigger);
      cy.get(trigger).then(($button) => {
        const style = getComputedStyle($button[0]);
        for (const side of [style.marginTop, style.marginRight, style.marginBottom, style.marginLeft]) {
          expect(parseFloat(side)).to.be.within(3, 6);
        }
      });
    });
  }
});

const status = "header .topbar-inner > [role=\"status\"]";
const deckHeaders = {
  "Content-Type": "application/octet-stream",
  "Content-Disposition": 'attachment; filename="arabic-deck-2026-01-01.apkg"',
  "X-Card-Count": "1",
};

function approvedCard() {
  cy.visit("/open?rank=4100");
  cy.location("pathname").then((card) => cy.request("POST", `/api${card.replace("/cards", "/notes")}/approve`, {}));
}

describe("export from the menu", () => {
  it("is disabled while nothing is approved", () => {
    cy.request("/api/backups").its("body.check.approved").then((approved: number) => {
      if (approved > 0) return;
      openMenu();
      cy.contains(`${popover} .options-item`, /^Export$/).should("be.disabled");
    });
  });

  it("is enabled once a card is approved", () => {
    approvedCard();
    openMenu();
    cy.contains(`${popover} .options-item`, /^Export$/).should("not.be.disabled");
  });

  it("asks the server once, hands the deck to the browser and writes nothing to the workspace", () => {
    approvedCard();
    cy.visit("/");
    cy.task<string[]>("downloads").then((before) => {
      cy.intercept("GET", "/api/export").as("export");
      cy.menuItem("Export");
      cy.get(status).invoke("text").should("match", /^Exported \d+ cards?\.$/);
      cy.get("@export.all").should("have.length", 1);
      cy.task<string[]>("downloads", before.length + 1).should((after) => expect(after).to.have.length(before.length + 1));
      cy.task<string[]>("workspaceExports").should("deep.equal", []);
    });
  });

  it("says Preparing the deck… and then Exported 1 card.", () => {
    approvedCard();
    cy.visit("/");
    cy.intercept("GET", "/api/export", { delay: 600, headers: deckHeaders, body: "x" });
    cy.menuItem("Export");
    cy.get(status).should("have.text", "Preparing the deck…");
    cy.get(status).should("have.text", "Exported 1 card.");
  });

  it("says why when the export is refused and saves nothing", () => {
    approvedCard();
    cy.visit("/");
    cy.task<string[]>("downloads").then((before) => {
      cy.intercept("GET", "/api/export", { statusCode: 400, body: { error: "The card of the word X has no audio file." } });
      cy.menuItem("Export");
      cy.get(status).should("have.text", "The card of the word X has no audio file.");
      cy.task<string[]>("downloads").should("deep.equal", before);
    });
  });

  it("says Request failed (500). when the server answers with something that is not JSON", () => {
    approvedCard();
    cy.visit("/");
    cy.intercept("GET", "/api/export", { statusCode: 500, headers: { "Content-Type": "text/html" }, body: "<!DOCTYPE html><p>oops</p>" });
    cy.menuItem("Export");
    cy.get(status).should("have.text", "Request failed (500).");
  });

  it("says Request failed (400). when the refusal carries no message", () => {
    approvedCard();
    cy.visit("/");
    cy.intercept("GET", "/api/export", { statusCode: 400, body: {} });
    cy.menuItem("Export");
    cy.get(status).should("have.text", "Request failed (400).");
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
