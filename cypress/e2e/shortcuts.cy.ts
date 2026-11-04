export {};

const help = 'button[aria-label="Keyboard shortcuts"]';

describe("the list of shortcuts", () => {
  it("opens from the ? button and closes with its Close button", () => {
    cy.visit("/");
    cy.hydrated(help);
    cy.get("dialog[open]").should("not.exist");
    cy.get(help).click();
    cy.get("dialog[open]").should("contain", "Approve and go to the next word").and("contain", "Generate an example");
    cy.get("dialog[open]").contains("button", "Close").click();
    cy.get("dialog[open]").should("not.exist");
  });

  it("opens with the ? key and closes with Escape", () => {
    cy.visit("/");
    cy.hydrated(help);
    cy.get("body").type("?");
    cy.get("dialog[open]").should("be.visible");
    cy.get("body").type("{esc}");
    cy.get("dialog[open]").should("not.exist");
  });

  it("leaves single keys alone while you are typing in a field", () => {
    cy.visit("/search");
    cy.hydrated(help);
    cy.get('input[aria-label="Search"]').type("a/b?c");
    cy.get('input[aria-label="Search"]').should("have.value", "a/b?c");
    cy.location("pathname").should("eq", "/search");
    cy.get("dialog[open]").should("not.exist");
  });
});
