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
});
