export {};

const opener = 'button:contains("Backups")';
const dialog = "dialog[open]";
const rows = `${dialog} ol[aria-label="Backups"] > li`;

const clipBytes = (rank: number) => new Uint8Array([0xff, 0xfb, 0x90, 0x00, 0x55, rank % 256, rank >> 8, 0x55]);

function pressEscape() {
  for (const type of ["keyDown", "keyUp"]) {
    cy.then(() => Cypress.automation("remote:debugger:protocol", {
      command: "Input.dispatchKeyEvent",
      params: { type, key: "Escape", code: "Escape", windowsVirtualKeyCode: 27 },
    }));
  }
}

function openDialog() {
  cy.visit("/");
  cy.hydrated(opener);
  cy.get(opener).click();
  cy.contains(`${dialog} button`, "Back up now");
}

function backUp() {
  cy.get("body").then(($body) => {
    const before = $body.find(rows).length;
    cy.contains(`${dialog} button`, "Back up now").click();
    cy.get(rows).should("have.length", before + 1);
  });
}

describe("backups", () => {
  it("adds a row when Back up now is pressed", () => {
    openDialog();
    backUp();
  });

  it("brings back a card that was reset, and Cancel changes nothing", () => {
    cy.visit("/open?rank=411");
    cy.location("pathname").then((path) => {
      const id = path.split("/").pop()!;
      openDialog();
      backUp();
      cy.request("DELETE", `/api/notes/${id}`);
      openDialog();
      cy.contains(rows, "Backed up by you").contains("button", "Restore…").click();
      cy.focused().should("contain.text", "Cancel");
      cy.get(dialog).should("contain", "Your current state is saved first");
      cy.get(dialog).invoke("text").should("match", /It has \d+ cards? \(\d+ approved\); the workspace has \d+ \(\d+ approved\) now/);
      cy.contains(`${dialog} button`, "Cancel").click();
      cy.request({ url: `/cards/${id}`, failOnStatusCode: false }).its("status").should("eq", 404);
      cy.contains(rows, "Backed up by you").contains("button", "Restore…").click();
      cy.contains(`${dialog} button`, "Restore").click();
      cy.get(dialog).should("not.exist");
      cy.request(`/cards/${id}`).its("status").should("eq", 200);
    });
  });

  it("deletes a backup only after asking, and Cancel keeps it", () => {
    openDialog();
    backUp();
    cy.get(rows).its("length").then((before) => {
      cy.contains(rows, "Backed up by you").contains("button", "Delete…").click();
      cy.get(dialog).should("contain", "It cannot be brought back");
      cy.contains(`${dialog} button`, "Cancel").click();
      cy.get(rows).should("have.length", before);
      cy.contains(rows, "Backed up by you").contains("button", "Delete…").click();
      cy.contains(`${dialog} button`, "Delete").click();
      cy.get(rows).should("have.length", before - 1);
    });
  });

  for (const [width, height] of [[1920, 1100], [390, 844]]) {
    it(`fits the footer and the dialog at ${width}px`, () => {
      cy.viewport(width, height);
      cy.visit("/");
      cy.hydrated(opener);
      cy.get(opener).scrollIntoView().should(($button) => {
        const box = $button[0].getBoundingClientRect();
        expect(box.left).to.be.at.least(0);
        expect(box.right).to.be.at.most($button[0].ownerDocument.documentElement.clientWidth);
      });
      cy.document().should((doc) => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
      cy.get(opener).click();
      cy.contains(`${dialog} button`, "Back up now");
      backUp();
      cy.get(dialog).should(($dialog) => {
        const box = $dialog[0].getBoundingClientRect();
        expect(box.left).to.be.at.least(0);
        expect(box.right).to.be.at.most($dialog[0].ownerDocument.documentElement.clientWidth);
        expect($dialog[0].scrollWidth).to.be.at.most($dialog[0].clientWidth);
      });
    });
  }

  it("closes on Escape, gives the focus back to the button that opened it, and opens again", () => {
    openDialog();
    pressEscape();
    cy.get(dialog).should("not.exist");
    cy.focused().should("contain.text", "Backups");
    cy.get(opener).click();
    cy.get(dialog);
  });

  it("lists a clip whose audio file is gone, and nothing once the clip is removed", () => {
    cy.visit("/open?rank=412");
    cy.location("pathname").then((path) => {
      const id = path.split("/").pop()!;
      cy.window()
        .then(async (win) => {
          const res = await win.fetch(`/api/audio/${id}/word`, { method: "PUT", body: new win.Uint8Array(clipBytes(412)) });
          return (await res.json()).note.audio.word.key as string;
        })
        .then((key) => cy.task("removeAudio", key));
      openDialog();
      cy.get(dialog).find(`a[href="/cards/${id}"]`).should("exist");
      cy.request("DELETE", `/api/audio/${id}/word`);
      openDialog();
      cy.get(dialog).find(`a[href="/cards/${id}"]`).should("not.exist");
    });
  });
});
