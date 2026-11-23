export {};

const dialog = "dialog[open]";

const clipBytes = (rank: number, length: number) => {
  const bytes = new Uint8Array(length).fill(0x55);
  bytes.set([0xff, 0xfb, 0x90, 0x00, 0x77, rank % 256, rank >> 8, 0x77]);
  return bytes;
};

const unused = () => cy.request("/api/audio/unused").its("body");

/** Leaves an audio file in the workspace that no clip uses: the word's clip, uploaded and removed again. */
function unusedFile(rank: number, length = 8) {
  cy.visit(`/open?rank=${rank}`);
  cy.location("pathname").then((path) => {
    const url = `/api/audio/${path.split("/").pop()}/word`;
    cy.window().then(async (win) => {
      await win.fetch(url, { method: "PUT", body: new win.Uint8Array(clipBytes(rank, length)) });
      await win.fetch(url, { method: "DELETE" });
    });
  });
}

function openDialog() {
  cy.visit("/");
  cy.menuItem("Prune audio");
  cy.get(dialog);
}

describe("deleting the audio that no clip uses", () => {
  beforeEach(() => {
    cy.request("DELETE", "/api/audio/unused");
  });

  it("says how many files there are, keeps them on Cancel and deletes them on Delete", () => {
    unusedFile(301);
    unusedFile(302);
    unused().should("deep.equal", { files: 2, bytes: 16 });
    openDialog();
    cy.get(dialog).should("contain", "Delete 2 unused audio files (16 B)?").and("contain", "Cards keep every clip they use.");
    cy.get(dialog).contains("button", "Delete").should("have.css", "background-color", "rgb(193, 63, 58)");
    cy.get(dialog).contains("button", "Cancel").should("have.css", "background-color", "rgba(0, 0, 0, 0)");
    cy.get(dialog).contains("button", "Cancel").click();
    cy.get(dialog).should("not.exist");
    unused().should("deep.equal", { files: 2, bytes: 16 });
    cy.menuItem("Prune audio");
    cy.get(dialog).contains("button", "Delete").click();
    cy.get(dialog).should("contain", "Deleted 2 audio files (16 B).");
    unused().should("deep.equal", { files: 0, bytes: 0 });
    cy.get(dialog).contains("button", "Close").click();
    cy.get(dialog).should("not.exist");
    cy.menuItem("Prune audio");
    cy.get(dialog).should("contain", "No unused audio files.");
  });

  it("says file, not files, for one, and the size in KB", () => {
    unusedFile(303, 2048);
    openDialog();
    cy.get(dialog).should("contain", "Delete 1 unused audio file (2 KB)?");
  });

  it("gives a large size in MB", () => {
    unusedFile(307, 1.5 * 1024 * 1024);
    openDialog();
    cy.get(dialog).should("contain", "Delete 1 unused audio file (1.5 MB)?");
    cy.get(dialog).contains("button", "Delete").click();
    cy.get(dialog).should("contain", "Deleted 1 audio file (1.5 MB).");
  });

  for (const [width, height] of [[1920, 1100], [390, 844]]) {
    it(`fits the dialog at ${width}px`, () => {
      cy.viewport(width, height);
      unusedFile(306);
      cy.visit("/");
      cy.document().should((doc) => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
      cy.menuItem("Prune audio");
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
    unusedFile(308);
    openDialog();
    cy.get(dialog).contains("button", "Cancel").click();
    closedWithFocusOnMenu();
    cy.menuItem("Prune audio");
    cy.get(dialog).contains("button", "Delete").click();
    cy.get(dialog).contains("button", "Close").click();
    closedWithFocusOnMenu();
    cy.menuItem("Prune audio");
    cy.get(dialog).should("contain", "No unused audio files.");
    cy.get(dialog).contains("button", "Delete").should("not.exist");
    cy.pressEscape();
    closedWithFocusOnMenu();
    cy.menuItem("Prune audio");
    cy.get(dialog).should("contain", "No unused audio files.");
    cy.clickBackdrop();
    closedWithFocusOnMenu();
    cy.menuItem("Prune audio");
    cy.get(dialog);
  });
});
