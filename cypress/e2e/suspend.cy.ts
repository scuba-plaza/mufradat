export {};

const rows = '[aria-label="Word list"] > li';
const bar = '[role="toolbar"]';
const suspendButton = '[title="Suspend word"]';
const unsuspendButton = '[title="Unsuspend word"]';
const dialog = 'dialog[open]';
const mp3 = [0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4];

/** Opens the card of a word, once the page is ready to take input. */
function openWord(rank: number) {
  cy.visit(`/open?rank=${rank}`);
  cy.hydrated(suspendButton);
}

/** Fills a card out over the API, so it has everything an approval needs. */
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
    finishCard(32);
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
    openWord(33);
    cy.get(suspendButton).click();
    cy.get(unsuspendButton).should("be.visible");
    cy.visit(`/open?rank=34`);
    cy.hydrated('[aria-label="Arabic headword"]');
    cy.get("body").type("{ctrl}{leftarrow}");
    cy.contains("Word 32 of 5000");
  });
});
