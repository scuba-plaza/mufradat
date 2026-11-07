export {};

const by = (label: string) => `[aria-label="${label}"]`;
const unplayable = { contents: Cypress.Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]), fileName: "clip.mp3", mimeType: "audio/mpeg" };

function openWord(rank: number) {
  cy.visit(`/open?rank=${rank}`);
  cy.hydrated(by("Hint"));
}

describe("the keys of a card", () => {
  it("goes to the next word with Ctrl+→", () => {
    openWord(90);
    cy.get("body").type("{ctrl}{rightarrow}");
    cy.contains("Word 91 of 5000");
  });

  it("goes to the previous word with Ctrl+←", () => {
    openWord(92);
    cy.get("body").type("{ctrl}{leftarrow}");
    cy.contains("Word 91 of 5000");
  });

  it("leaves the card it goes from as it was", () => {
    openWord(93);
    cy.location("pathname").then((card) => {
      cy.get("body").type("{ctrl}{rightarrow}");
      cy.contains("Word 94 of 5000");
      cy.visit(card);
      cy.contains("button", /^Approve$/);
    });
  });

  it("stays on the first word on Ctrl+←, and on the last on Ctrl+→", () => {
    cy.intercept("/api/step*").as("step");
    for (const [rank, keys] of [[1, "{ctrl}{leftarrow}"], [5000, "{ctrl}{rightarrow}"]] as const) {
      openWord(rank);
      cy.location("pathname").then((card) => {
        cy.get("body").type(keys);
        cy.wait("@step").its("response.body").should("deep.equal", { href: null });
        cy.contains(`Word ${rank} of 5000`);
        cy.location("pathname").should("eq", card);
      });
    }
  });

  it("plays the word with Ctrl+↑", () => {
    openWord(94);
    cy.get(`${by("Audio for the word")} input[type="file"]`).selectFile(unplayable, { force: true });
    cy.get('[aria-label="Audio for the word"] [title="Remove"]').should("be.enabled");
    cy.get("body").type("{ctrl}{uparrow}");
    cy.contains("could not be played");
  });

  it("plays the example with Ctrl+↓", () => {
    openWord(95);
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).type("الْكِتَابُ [فِي] الْحَقِيبَةِ").blur();
    cy.get(`${by("Audio for the example")} input[type="file"]`).selectFile(unplayable, { force: true });
    cy.get('[aria-label="Audio for the example"] [title="Remove"]').should("be.enabled");
    cy.get("body").type("{ctrl}{downarrow}");
    cy.contains("could not be played");
  });

  it("does not act on them while a field has focus", () => {
    cy.intercept("/api/step*").as("step");
    openWord(96);
    cy.location("pathname").then((card) => {
      cy.get(by("Hint")).type("a hint{ctrl}{rightarrow}");
      cy.wait(300);
      cy.get("@step.all").should("have.length", 0);
      cy.location("pathname").should("eq", card);
      cy.get(by("Hint")).should("have.value", "a hint");
    });
  });

  it("does not act on them while a dialog is open", () => {
    cy.intercept("/api/step*").as("step");
    openWord(97);
    cy.location("pathname").then((card) => {
      cy.get(`${by("Forms")} [aria-haspopup="dialog"]`).click();
      cy.get("dialog[open]");
      cy.get("body").type("{ctrl}{rightarrow}");
      cy.wait(300);
      cy.get("@step.all").should("have.length", 0);
      cy.location("pathname").should("eq", card);
    });
  });
});

describe("the list of keys", () => {
  it("opens from the menu, lists each key, and closes with Close", () => {
    cy.visit("/");
    cy.get("dialog[open]").should("not.exist");
    cy.menuItem("Keyboard shortcuts");
    cy.get("dialog[open]").within(() => {
      cy.contains("Previous word").parent().should("contain", "Ctrl").and("contain", "←");
      cy.contains("Next word").parent().should("contain", "Ctrl").and("contain", "→");
      cy.contains("Play the word").parent().should("contain", "Ctrl").and("contain", "↑");
      cy.contains("Play the example").parent().should("contain", "Ctrl").and("contain", "↓");
      cy.contains("button", "Close").click();
    });
    cy.get("dialog[open]").should("not.exist");
  });
});

describe("the keys of a card opened from a list", () => {
  function openFrom(query: string, rank: number) {
    cy.visit(`/?q=${encodeURIComponent(query)}`);
    cy.wordRow(rank).click();
    cy.hydrated(by("Hint"));
  }

  it("walks the list it was opened from instead of the frequency list", () => {
    openFrom("rank:181-185 sort:-rank", 184);
    cy.contains("Word 184 of 5000");
    cy.get("body").type("{ctrl}{rightarrow}");
    cy.contains("Word 183 of 5000");
    cy.hydrated(by("Hint"));
    cy.get("body").type("{ctrl}{leftarrow}");
    cy.contains("Word 184 of 5000");
  });

  it("goes on to the next word from a word that was just started in the list of the words not started", () => {
    openFrom("status:todo rank:195-199", 197);
    cy.contains("Word 197 of 5000");
    cy.get("body").type("{ctrl}{rightarrow}");
    cy.contains("Word 198 of 5000");
  });

  it("goes back to the previous word from a word that was just started in the list of the words not started", () => {
    openFrom("status:todo rank:175-179", 177);
    cy.contains("Word 177 of 5000");
    cy.get("body").type("{ctrl}{leftarrow}");
    cy.contains("Word 176 of 5000");
  });

  it("stays on the last word of the list it was opened from", () => {
    cy.intercept("/api/step*").as("step");
    openFrom("rank:191-192", 192);
    cy.location("pathname").then((card) => {
      cy.get("body").type("{ctrl}{rightarrow}");
      cy.wait("@step").its("response.body").should("deep.equal", { href: null });
      cy.contains("Word 192 of 5000");
      cy.location("pathname").should("eq", card);
    });
  });
});
