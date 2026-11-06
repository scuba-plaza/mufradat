export {};

const dialog = "dialog[open]";

const clipBytes = (rank: number, length: number) => {
  const bytes = new Uint8Array(length).fill(0x55);
  bytes.set([0xff, 0xfb, 0x90, 0x00, 0x77, rank % 256, rank >> 8, 0x77]);
  return bytes;
};

const unused = () => cy.request("/api/audio/unused").its("body");

function withClip(rank: number, keep: boolean, length = 8) {
  cy.visit(`/open?rank=${rank}`);
  return cy.location("pathname").then((path) => {
    const id = path.split("/").pop()!;
    cy.window().then(async (win) => {
      const url = `/api/audio/${id}/word`;
      await win.fetch(url, { method: "PUT", body: new win.Uint8Array(clipBytes(rank, length)) });
      if (!keep) await win.fetch(url, { method: "DELETE" });
    });
    return cy.wrap(id);
  });
}

function openDialog() {
  cy.visit("/");
  cy.menuItem("Delete unused audio");
  cy.get(dialog);
}

describe("deleting the audio that no clip uses", () => {
  beforeEach(() => {
    cy.request("DELETE", "/api/audio/unused");
  });

  it("says how many files there are, keeps them on Cancel and deletes them on Delete", () => {
    withClip(301, false);
    withClip(302, false);
    unused().should("deep.equal", { files: 2, bytes: 16 });
    openDialog();
    cy.get(dialog).should("contain", "Delete 2 unused audio files (16 B)?").and("contain", "Cards keep every clip they use.");
    cy.get(dialog).contains("button", "Cancel").click();
    cy.get(dialog).should("not.exist");
    unused().should("deep.equal", { files: 2, bytes: 16 });
    cy.menuItem("Delete unused audio");
    cy.get(dialog).contains("button", "Delete").click();
    cy.get(dialog).should("contain", "Deleted 2 audio files (16 B).");
    unused().should("deep.equal", { files: 0, bytes: 0 });
    cy.get(dialog).contains("button", "Close").click();
    cy.get(dialog).should("not.exist");
    cy.menuItem("Delete unused audio");
    cy.get(dialog).should("contain", "No unused audio files.");
  });

  it("says file, not files, for one, and the size in KB", () => {
    withClip(303, false, 2048);
    openDialog();
    cy.get(dialog).should("contain", "Delete 1 unused audio file (2 KB)?");
  });

  it("gives a large size in MB", () => {
    withClip(307, false, 1.5 * 1024 * 1024);
    openDialog();
    cy.get(dialog).should("contain", "Delete 1 unused audio file (1.5 MB)?");
    cy.get(dialog).contains("button", "Delete").click();
    cy.get(dialog).should("contain", "Deleted 1 audio file (1.5 MB).");
  });

  it("leaves a clip that a card still has playing", () => {
    withClip(304, true).then((kept) => {
      withClip(305, false);
      openDialog();
      cy.get(dialog).should("contain", "Delete 1 unused audio file");
      cy.get(dialog).contains("button", "Delete").click();
      cy.get(dialog).should("contain", "Deleted 1 audio file (8 B).");
      cy.request(`/api/audio/${kept}/word`).its("headers.content-type").should("eq", "audio/mpeg");
    });
  });

  it("only says so when there is nothing to delete, with nothing to confirm", () => {
    openDialog();
    cy.get(dialog).should("contain", "No unused audio files.");
    cy.get(dialog).contains("button", "Delete").should("not.exist");
    cy.get(dialog).contains("button", "Close").click();
    cy.get(dialog).should("not.exist");
  });

  for (const [width, height] of [[1920, 1100], [390, 844]]) {
    it(`fits the dialog at ${width}px`, () => {
      cy.viewport(width, height);
      withClip(306, false);
      cy.visit("/");
      cy.document().should((doc) => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
      cy.menuItem("Delete unused audio");
      cy.get(dialog).should(($dialog) => {
        const box = $dialog[0].getBoundingClientRect();
        expect(box.left).to.be.at.least(0);
        expect(box.right).to.be.at.most(Cypress.config("viewportWidth"));
      });
    });
  }

  it("gives the focus back to the menu button after Cancel, Close, Escape and a click on the backdrop, and opens again each time", () => {
    const closedWithFocusOnMenu = () => {
      cy.get(dialog).should("not.exist");
      cy.focused().should("have.attr", "aria-label", "Menu");
    };
    withClip(308, false);
    openDialog();
    cy.get(dialog).contains("button", "Cancel").click();
    closedWithFocusOnMenu();
    cy.menuItem("Delete unused audio");
    cy.get(dialog).contains("button", "Delete").click();
    cy.get(dialog).contains("button", "Close").click();
    closedWithFocusOnMenu();
    cy.menuItem("Delete unused audio");
    cy.get(dialog).should("contain", "No unused audio files.");
    cy.pressEscape();
    closedWithFocusOnMenu();
    cy.menuItem("Delete unused audio");
    cy.get(dialog).should("contain", "No unused audio files.");
    cy.clickBackdrop();
    closedWithFocusOnMenu();
    cy.menuItem("Delete unused audio");
    cy.get(dialog);
  });
});
