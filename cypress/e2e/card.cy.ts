export {};

const status = '[role="status"]';
const alert = '[role="alert"]';
const bar = '[role="toolbar"]';
const by = (label: string) => `[aria-label="${label}"]`;
/** An MP3 frame header and nothing playable after it: the store accepts it, a browser cannot decode it. */
const unplayable = { contents: Cypress.Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]), fileName: "word.mp3", mimeType: "audio/mpeg" };

/** Opens the card of a word, once the page is ready to take input. */
function openWord(rank: number) {
  cy.visit(`/open?rank=${rank}`);
  cy.hydrated(by("Hint"));
}

describe("a word", () => {
  it("is titled with its rank", () => {
    cy.visit("/open?rank=5");
    cy.title().should("eq", "Word 5 · Mufradat");
  });

  it("shows the card the way it will be learned", () => {
    openWord(3);
    cy.get(by("Arabic headword")).should("have.value", "عَلَى");
    cy.get(by("English meaning")).should("have.value", "on, over");
    cy.contains("preposition");
    cy.contains("Word 3 of 5000");
  });

  it("says Saving… and then Saved when a field is edited", () => {
    cy.intercept("PUT", "/api/notes/*", (req) => req.continue((res) => {
      res.setDelay(600);
    }));
    openWord(8);
    cy.get(by("Hint")).type("a hint").blur();
    cy.get(status).should("contain", "Saving…");
    cy.get(status).should("contain", "Saved");
  });

  it("says when a change was not saved, and why", () => {
    cy.intercept("PUT", "/api/notes/*", { statusCode: 500, body: { error: "The disk is full." } });
    openWord(9);
    cy.get(by("Hint")).type("a hint").blur();
    cy.get(status).should("contain", "Not saved");
    cy.get(alert).should("contain", "The disk is full.");
  });

  it("edits the meaning where it is shown, and keeps it", () => {
    openWord(14);
    cy.get(by("English meaning")).clear().type("a test meaning").blur();
    cy.get(status).should("contain", "Saved");
    cy.reload();
    cy.get(by("English meaning")).should("have.value", "a test meaning");
  });

  it("shows the example with its target word marked, and edits it where it is shown", () => {
    openWord(15);
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).type("الْكِتَابُ [فِي] الْحَقِيبَةِ").blur();
    cy.get(`${by("Example")} mark`).should("have.text", "فِي");
    cy.get(status).should("contain", "Saved");
    cy.reload();
    cy.get(`${by("Example")} mark`).should("have.text", "فِي");
  });

  it("marks the selected text of the example as its target", () => {
    openWord(16);
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).type("الْكِتَابُ فِي الْحَقِيبَةِ").then(($box) => {
      const text = $box.val() as string;
      const start = text.indexOf("فِي");
      ($box[0] as HTMLTextAreaElement).setSelectionRange(start, start + "فِي".length);
    });
    cy.contains("button", "Mark target").click();
    cy.get(`textarea${by("Example")}`).should("have.value", "الْكِتَابُ [فِي] الْحَقِيبَةِ").blur();
    cy.get(`${by("Example")} mark`).should("have.text", "فِي");
  });

  it("says when it is not clear which word of the example to highlight, until you mark it", () => {
    openWord(23);
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).type("الْكِتَابُ فِي الْحَقِيبَةِ").then(($box) => {
      const text = $box.val() as string;
      const start = text.indexOf("فِي");
      ($box[0] as HTMLTextAreaElement).setSelectionRange(start, start + "فِي".length);
    });
    cy.get(`textarea${by("Example")}`).blur();
    cy.contains("It is not clear which word to highlight");
    cy.get(bar).should("contain", "the target word marked in the example");
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).then(($box) => {
      const start = ($box.val() as string).indexOf("فِي");
      ($box[0] as HTMLTextAreaElement).setSelectionRange(start, start + "فِي".length);
    });
    cy.contains("button", "Mark target").click();
    cy.get(`textarea${by("Example")}`).blur();
    cy.get(`${by("Example")} mark`).should("have.text", "فِي");
    cy.contains("It is not clear which word to highlight").should("not.exist");
    cy.get(bar).should("not.contain", "the target word marked in the example");
  });

  it("keeps a failed example under the example and lets you try again", () => {
    openWord(17);
    cy.contains("button", "Generate example").click();
    cy.contains("There is no Mistral API key");
    cy.contains("button", "Generate example").should("be.enabled");
  });

  it("keeps a failed clip under its text and lets you try again", () => {
    openWord(18);
    cy.get(by("Audio for the word")).contains("button", "Generate").click();
    cy.contains("There are no Google credentials");
    cy.get(by("Audio for the word")).contains("button", "Generate").should("be.enabled");
  });

  it("says when a clip cannot be played", () => {
    openWord(19);
    cy.get(`${by("Audio for the word")} input[type="file"]`).selectFile(unplayable, { force: true });
    cy.get(by("Play the word")).click();
    cy.contains("could not be played");
  });

  it("plays the word with the w key", () => {
    openWord(20);
    cy.get(`${by("Audio for the word")} input[type="file"]`).selectFile(unplayable, { force: true });
    cy.get(by("Play the word"));
    cy.get("body").type("w");
    cy.contains("could not be played");
  });

  it("keeps the lexical fields in Details, closed until you open them", () => {
    openWord(21);
    cy.get("details").should("not.have.attr", "open");
    cy.contains("label", "Part of speech").should("not.be.visible");
    cy.contains("summary", "Details").click();
    cy.contains("label", "Part of speech").should("be.visible");
    cy.contains("label", "Gender");
    cy.contains("label", "Root");
    cy.contains("label", "Verb form");
    cy.contains("label", "Part of speech").find("select").select("adverb");
    cy.get(status).should("contain", "Saved");
  });

  it("says what the card still needs, in the action bar", () => {
    openWord(22);
    cy.get(bar).should("contain", "Still needs an example, audio for the word");
    cy.get(`${by("Audio for the word")} input[type="file"]`).selectFile(unplayable, { force: true });
    cy.get(bar).should("contain", "Still needs an example").and("not.contain", "audio for the word");
  });

  it("approves the card and goes on to the next word", () => {
    cy.visit("/open?rank=60");
    cy.hydrated(by("Hint"));
    cy.location("pathname").then((card) => {
      cy.contains("button", "Approve and next").click();
      cy.location("pathname").should("not.eq", card).and("match", /^\/cards\/[0-9a-f-]{36}$/);
      cy.visit(card);
      cy.get(bar).should("contain", "approved");
      cy.contains("button", "Approve and next").should("not.exist");
      cy.contains("a", "Next").should("have.attr", "href", "/next");
    });
  });

  it("stays on the card and says why when it cannot be approved", () => {
    openWord(61);
    cy.location("pathname").then((card) => {
      cy.get(by("English meaning")).clear().blur();
      cy.contains("button", "Approve and next").click();
      cy.get(alert).should("contain", "The English meaning is empty.");
      cy.location("pathname").should("eq", card);
    });
  });

  it("approves and goes on with Ctrl+Enter, even from inside a field", () => {
    openWord(62);
    cy.location("pathname").then((card) => {
      cy.get(by("Hint")).type("{ctrl}{enter}");
      cy.location("pathname").should("not.eq", card).and("match", /^\/cards\/[0-9a-f-]{36}$/);
    });
  });

  it("skips the word and opens another", () => {
    openWord(63);
    cy.location("pathname").then((card) => {
      cy.contains("button", "Skip").click();
      cy.location("pathname").should("not.eq", card).and("match", /^\/cards\/[0-9a-f-]{36}$/);
    });
  });

  it("treats single keys as letters while you type, and as shortcuts otherwise", () => {
    openWord(64);
    cy.get(by("Hint")).type("gds{enter}");
    cy.get(by("Hint")).should("have.value", "gds");
    cy.contains("There is no Mistral API key").should("not.exist");
    cy.get("details").should("not.have.attr", "open");
    cy.get("body").type("d");
    cy.get("details").should("have.attr", "open");
    cy.get("body").type("g");
    cy.contains("There is no Mistral API key");
  });

  it("shows a friendly page for a card that does not exist", () => {
    cy.visit("/cards/nope", { failOnStatusCode: false });
    cy.contains("There is nothing at this address.");
    cy.contains("a", "Back to the word list").should("have.attr", "href", "/");
  });
});
