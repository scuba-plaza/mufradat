export {};

const rows = '[aria-label="Word list"] > li';
const bar = '[role="toolbar"]';
const suspendButton = '[title="Suspend word"]';
const unsuspendButton = '[title="Unsuspend word"]';
const dialog = 'dialog[open]';

/** Opens the card of a word, once the page is ready to take input. */
function openWord(rank: number) {
  cy.visit(`/open?rank=${rank}`);
  cy.hydrated(suspendButton);
}

describe("suspending a word", () => {
  it("parks a card that is not approved, and only status:suspended finds it", () => {
    openWord(30);
    cy.get('[aria-label="Arabic headword"]').invoke("val").then((value) => {
      const lemma = String(value);
      cy.get(suspendButton).click();
      cy.get(unsuspendButton).should("be.visible");
      cy.get(bar).should("contain", "suspended");
      cy.visit("/");
      cy.get(rows).contains(lemma).should("not.exist");
      cy.visit("/?q=status%3Asuspended");
      cy.get(rows).contains(lemma).should("exist");
    });
  });

  it("asks first when the card is approved, and takes the approval back", () => {
    cy.finishCard(32);
    cy.location("pathname").then((card) => {
      cy.request("POST", `/api/notes/${card.split("/").pop()}/approve`, {});
      cy.visit(card);
      cy.hydrated(suspendButton);
      cy.get(suspendButton).click();
      cy.get(dialog).contains("h2", "Suspend this word?").should("be.visible");
      cy.get(dialog).contains("button", "Cancel").click();
      cy.get(suspendButton).should("be.visible");
      cy.get(suspendButton).click();
      cy.get(dialog).contains("button", "Suspend").click();
      cy.get(unsuspendButton).should("be.visible");
      cy.get(bar).should("contain", "suspended");
      cy.get(bar).contains("button", "Unsuspend").should("be.visible");
      cy.get(bar).contains("button", "Approve").should("not.exist");
    });
  });

  it("comes back everywhere once it is unsuspended", () => {
    openWord(31);
    cy.get(suspendButton).click();
    cy.get(unsuspendButton).click();
    cy.get(suspendButton).should("be.visible");
    cy.visit("/");
    cy.wordRow(31).should("exist");
    cy.visit("/?q=status%3Asuspended");
    cy.get(rows).should("not.contain", "31");
  });

  it("keeps the keyboard arrows off the suspended card", () => {
    openWord(133);
    cy.get(suspendButton).click();
    cy.get(unsuspendButton).should("be.visible");
    cy.visit(`/open?rank=134`);
    cy.hydrated('[aria-label="Arabic headword"]');
    cy.get("body").type("{ctrl}{leftarrow}");
    cy.contains("Word 132 of 5000");
  });
});
