export {};

const trigger = 'button[aria-label="Menu"]';
const popover = "header [popover]";

function openMenu() {
  cy.visit("/");
  cy.hydrated(trigger);
  cy.get(trigger).click();
}

describe("the menu", () => {
  it("opens from the ⋯ button in the header and lists Backups, Prune audio and Keyboard shortcuts in that order, none of them on the page", () => {
    cy.visit("/");
    cy.hydrated(trigger);
    cy.get(popover).should("not.be.visible");
    cy.get(trigger).click();
    cy.get(popover).should("be.visible");
    cy.get(`${popover} .options-item`).then(($items) => {
      expect([...$items].map((item) => item.textContent)).to.deep.equal(["Backups", "Prune audio", "Keyboard shortcuts"]);
    });
    cy.get("main").should("not.contain", "Backups").and("not.contain", "Prune audio");
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
    it(`puts the button with a small margin at the right end of the top bar, and keeps the menu inside the window, level with it, at ${width}px`, () => {
      cy.viewport(width, height);
      openMenu();
      cy.get(popover).should("be.visible");
      cy.get(trigger).then(($button) => {
        const style = getComputedStyle($button[0]);
        for (const side of [style.marginTop, style.marginRight, style.marginBottom, style.marginLeft]) {
          expect(parseFloat(side)).to.be.within(3, 6);
        }
        cy.get("header .topbar-inner").then(($bar) => {
          const bar = $bar[0].getBoundingClientRect();
          const inside = bar.right - parseFloat(getComputedStyle($bar[0]).paddingRight) - parseFloat(style.marginRight);
          expect(Math.abs($button[0].getBoundingClientRect().right - inside)).to.be.at.most(1);
        });
        cy.get(popover).then(($popover) => {
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

  it("gives the focus back to the menu button when the keyboard shortcuts close with Close, Escape or a click on the backdrop", () => {
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
    cy.menuItem("Keyboard shortcuts");
    cy.clickBackdrop();
    cy.get("dialog[open]").should("not.exist");
    cy.focused().should("have.attr", "aria-label", "Menu");
  });

  it("shows the menu on the not-found page and opens it", () => {
    cy.visit("/cards/nope", { failOnStatusCode: false });
    cy.hydrated(trigger).click();
    cy.get(popover).should("be.visible").and("contain", "Keyboard shortcuts");
  });
});
