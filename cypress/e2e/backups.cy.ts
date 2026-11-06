export {};

const dialog = "dialog[open]";
const rows = `${dialog} ol[aria-label="Backups"] > li`;

const clipBytes = (rank: number) => new Uint8Array([0xff, 0xfb, 0x90, 0x00, 0x55, rank % 256, rank >> 8, 0x55]);

function openDialog() {
  cy.visit("/");
  cy.menuItem("Backups");
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
  it("adds a row when Back up now is pressed, and shows what the backups use of the limit", () => {
    openDialog();
    backUp();
    cy.get(dialog).should("contain", "Automatic backups use").and("contain", "of 1.0 GB");
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

  it("goes to the home page after a restore that removes the card it was started from", () => {
    openDialog();
    backUp();
    cy.visit("/open?rank=413");
    cy.location("pathname").should("match", /^\/cards\//);
    cy.menuItem("Backups");
    cy.contains(rows, "Backed up by you").contains("button", "Restore…").click();
    cy.contains(`${dialog} button`, "Restore").click();
    cy.location("pathname").should("eq", "/");
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

  it("downloads a backup as a file and then says when it was last downloaded", () => {
    openDialog();
    backUp();
    cy.contains(rows, "Backed up by you").contains("a", "Download").then(($link) => {
      const id = String($link.attr("href")).split("/").pop()!;
      cy.wrap($link).click();
      cy.readFile(`${Cypress.config("downloadsFolder")}/mufradat-backup-${id}.zip`, null).should("have.length.greaterThan", 0);
    });
    openDialog();
    cy.get(dialog).should("contain", "Last downloaded:").and("not.contain", "Last downloaded: never");
  });

  it("imports a backup larger than 10 MiB whole through the file picker", () => {
    interface Listed {
      id: string;
      reason: string;
      clips: number;
    }
    cy.visit("/open?rank=421");
    cy.location("pathname").then((path) => {
      const id = path.split("/").pop()!;
      cy.window().then(async (win) => {
        for (const [slot, marker] of [["word", 1], ["example", 2]] as const) {
          const bytes = new win.Uint8Array(6 * 1024 * 1024);
          bytes.set([0xff, 0xfb, 0x90, 0x00, marker]);
          const res = await win.fetch(`/api/audio/${id}/${slot}`, { method: "PUT", body: bytes });
          expect(res.ok).to.eq(true);
        }
      });
      openDialog();
      backUp();
      cy.request("/api/backups").its("body.snapshots").then((snapshots: Listed[]) => {
        const source = snapshots.find((s) => s.reason === "manual")!;
        cy.window().then(async (win) => {
          const zip = await (await win.fetch(`/api/backups/${source.id}`)).arrayBuffer();
          expect(zip.byteLength).to.be.greaterThan(10 * 1024 * 1024);
          const input = win.document.querySelector<HTMLInputElement>('input[type="file"]')!;
          const transfer = new win.DataTransfer();
          transfer.items.add(new win.File([zip], "backup.zip", { type: "application/zip" }));
          input.files = transfer.files;
          input.dispatchEvent(new win.Event("change", { bubbles: true }));
        });
        cy.contains(rows, "Imported");
        cy.request("/api/backups").its("body.snapshots").then((after: Listed[]) => {
          const imported = after.find((s) => s.reason === "imported")!;
          expect(imported.clips).to.eq(source.clips);
          cy.request("DELETE", `/api/backups/${imported.id}`);
          cy.request("DELETE", `/api/backups/${source.id}`);
          cy.request("DELETE", `/api/audio/${id}/word`);
          cy.request("DELETE", `/api/audio/${id}/example`);
        });
      });
    });
  });

  it("refuses cross-site requests on the import route and on another route, and answers a same-site import", () => {
    for (const url of ["/api/backups/import", "/api/backups"]) {
      cy.request({ method: "POST", url, headers: { Origin: "http://evil.example" }, body: "x", failOnStatusCode: false })
        .its("status")
        .should("eq", 403);
    }
    cy.request({ method: "POST", url: "/api/backups/import", body: "x", failOnStatusCode: false }).its("status").should("eq", 400);
  });

  for (const [width, height] of [[1920, 1100], [390, 844]]) {
    it(`fits the dialog at ${width}px`, () => {
      cy.viewport(width, height);
      cy.visit("/");
      cy.document().should((doc) => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
      cy.menuItem("Backups");
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

  it("closes on Escape, gives the focus back to the menu button, and opens again", () => {
    openDialog();
    cy.pressEscape();
    cy.get(dialog).should("not.exist");
    cy.focused().should("have.attr", "aria-label", "Menu");
    cy.menuItem("Backups");
    cy.get(dialog);
  });

  it("closes with Close and with a click on the backdrop, each giving the focus back to the menu button and leaving it openable", () => {
    openDialog();
    cy.contains(`${dialog} button`, "Close").click();
    cy.get(dialog).should("not.exist");
    cy.focused().should("have.attr", "aria-label", "Menu");
    cy.menuItem("Backups");
    cy.clickBackdrop();
    cy.get(dialog).should("not.exist");
    cy.focused().should("have.attr", "aria-label", "Menu");
    cy.menuItem("Backups");
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
