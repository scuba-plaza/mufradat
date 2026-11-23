export {};

const status = '[role="status"]';
const alert = '[role="alert"]';
const bar = '[role="toolbar"]';
const by = (label: string) => `[aria-label="${label}"]`;
const resetButton = '[title="Reset word"]';
const word = by("Audio for the word");
const example = by("Audio for the example");
const inGroup = (group: string, title: string) => `${group} [title="${title}"]`;
const at = (el: Element) => {
  const box = el.getBoundingClientRect();
  return [box.left, box.top + el.ownerDocument.defaultView!.scrollY, box.width, box.height];
};
const askForExample = () => cy.get(inGroup(example, "New example")).click();
const tagsButton = '[title="Tags and details"]';
/** An MP3 frame header and nothing playable after it: the store accepts it, a browser cannot decode it. */
const unplayable = { contents: Cypress.Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]), fileName: "word.mp3", mimeType: "audio/mpeg" };

describe("a word", () => {
  it("shows the card the way it will be learned, titled with its rank", () => {
    cy.openWord(3);
    cy.title().should("eq", "Word 3 · Mufradat");
    cy.get(by("Arabic headword")).should("have.value", "عَلَى");
    cy.get(by("English meaning")).should("have.value", "on, over");
    cy.contains("Word 3 of 5000");
    cy.contains("p", "preposition").should("not.exist");
    cy.get(tagsButton).click();
    cy.contains("label", "Part of speech").find("select option:selected").should("have.text", "preposition");
  });

  it("says Saving… and then Saved when a field is edited", () => {
    cy.intercept("PUT", "/api/notes/*", (req) => req.continue((res) => {
      res.setDelay(600);
    }));
    cy.openWord(8);
    cy.get(by("Hint English")).type("a hint").blur();
    cy.get(status).should("contain", "Saving…");
    cy.get(status).should("contain", "Saved");
  });

  it("says when a change was not saved, and why", () => {
    cy.intercept("PUT", "/api/notes/*", { statusCode: 500, body: { error: "The disk is full." } });
    cy.openWord(9);
    cy.get(by("Hint English")).type("a hint").blur();
    cy.get(status).should("contain", "Not saved");
    cy.get(alert).should("contain", "The disk is full.");
  });

  it("edits the meaning and a separate hint for each side of the card where they are shown, and keeps them", () => {
    cy.openWord(14);
    cy.get(by("English meaning")).clear().type("a test meaning").blur();
    cy.get(by("Hint Arabic")).type("ليس عَنْ").blur();
    cy.get(by("Hint English")).type("not about").blur();
    cy.get(status).should("contain", "Saved");
    cy.reload();
    cy.get(by("English meaning")).should("have.value", "a test meaning");
    cy.get(by("Hint Arabic")).should("have.value", "ليس عَنْ");
    cy.get(by("Hint English")).should("have.value", "not about");
  });

  it("shows the example with its target word marked, and edits it where it is shown", () => {
    cy.openWord(15);
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).type("الْكِتَابُ [فِي] الْحَقِيبَةِ").blur();
    cy.get(`${by("Example")} mark`).should("have.text", "فِي");
    cy.get(status).should("contain", "Saved");
    cy.reload();
    cy.get(`${by("Example")} mark`).should("have.text", "فِي");
  });

  it("lets Tab move on from the meaning through the hints and the example to its English translation", () => {
    cy.openWord(24);
    cy.get(by("English meaning")).focus();
    cy.press(Cypress.Keyboard.Keys.TAB);
    cy.focused().should("have.attr", "aria-label", "Hint English");
    cy.press(Cypress.Keyboard.Keys.TAB);
    cy.focused().should("match", `textarea${by("Example")}`);
    cy.press(Cypress.Keyboard.Keys.TAB);
    cy.focused().should("have.attr", "aria-label", "Example in English");
  });

  it("keeps what was typed in the example when Tab moves on", () => {
    cy.openWord(25);
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).type("الْكِتَابُ [فِي] الْحَقِيبَةِ");
    cy.press(Cypress.Keyboard.Keys.TAB);
    cy.get(status).should("contain", "Saved");
    cy.reload();
    cy.get(`${by("Example")} mark`).should("have.text", "فِي");
  });

  const selectInExample = (letters: string) =>
    cy.get(`textarea${by("Example")}`).focus().then(($box) => {
      const start = ($box.val() as string).indexOf(letters);
      ($box[0] as HTMLTextAreaElement).setSelectionRange(start, start + letters.length);
    });

  it("marks the headword of the card by itself when the example is typed", () => {
    cy.openWord(16);
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).type("تَحَدَّثْنَا عَنِ الْحَقِيبَةِ").blur();
    cy.get(`${by("Example")} mark`).should("have.text", "عَنِ");
  });

  it("marks the letters that were selected last when the field is left, and not before, and keeps the mark when its own letters are selected again", () => {
    cy.openWord(17);
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).type("الْكِتَابُ فِي الْحَقِيبَةِ");
    selectInExample("كِتَاب");
    selectInExample("حَقِيب");
    cy.get(`textarea${by("Example")}`).should("have.value", "الْكِتَابُ فِي الْحَقِيبَةِ").blur();
    cy.get(`${by("Example")} mark`).should("have.text", "حَقِيب");
    cy.get(by("Example")).click();
    selectInExample("حَقِيب");
    cy.get(`textarea${by("Example")}`).blur();
    cy.get(`${by("Example")} mark`).should("have.text", "حَقِيب");
  });

  it("says when it is not clear which word of the example to highlight, until you mark it", () => {
    cy.openWord(23);
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).type("الْكِتَابُ فِي الْحَقِيبَةِ").blur();
    cy.contains("It is not clear which word to highlight");
    cy.get(bar).should("contain", "the target word marked in the example");
    cy.get(by("Example")).click();
    selectInExample("فِي");
    cy.get(`textarea${by("Example")}`).blur();
    cy.get(`${by("Example")} mark`).should("have.text", "فِي");
    cy.contains("It is not clear which word to highlight").should("not.exist");
    cy.get(bar).should("not.contain", "the target word marked in the example");
  });

  it("shows a failed example under its field, moving nothing, lets you try again, and takes it away once you write an example yourself", () => {
    cy.openWord(27);
    cy.get(by("Example in English")).then(($english) => {
      const before = at($english[0])[1];
      askForExample();
      cy.contains("There is no Mistral API key");
      cy.get(by("Example in English")).should(($now) => expect(at($now[0])[1]).to.equal(before));
    });
    cy.get(inGroup(example, "New example")).should("be.enabled");
    cy.get(by("Example")).click();
    cy.get(`textarea${by("Example")}`).type("الْكِتَابُ [فِي] الْحَقِيبَةِ").blur();
    cy.get(status).should("contain", "Saved");
    cy.contains("There is no Mistral API key").should("not.exist");
  });

  it("takes away what stopped the approval once a new example fixes it", () => {
    cy.openWord(29);
    cy.location("pathname").then((path) => {
      cy.get(by("Example")).click();
      cy.get(`textarea${by("Example")}`).type("الْكِتَابُ [فِي] الْحَقِيبَةِ").blur();
      cy.get(status).should("contain", "Saved");
      cy.contains("button", /^Approve$/).click();
      cy.get(alert).should("contain", "The example has no English translation.");
      cy.request("PUT", `/api/notes/${path.split("/").pop()}`, { exampleEn: "The book is in the bag." }).then((response) => {
        cy.intercept("POST", "/api/mistral", { body: { note: response.body.note } });
      });
      askForExample();
      cy.get(by("Example in English")).should("have.value", "The book is in the bag.");
      cy.get(alert).should("not.exist");
    });
  });

  it("says when a clip cannot be played", () => {
    cy.openWord(19);
    cy.get(`${word} input[type="file"]`).selectFile(unplayable, { force: true });
    cy.get(inGroup(word, "Remove")).should("be.enabled");
    cy.get(inGroup(word, "Play the word")).click();
    cy.contains("could not be played");
  });

  describe("the audio controls", () => {
    const mine = { ...unplayable, contents: Cypress.Buffer.from([0xff, 0xfb, 0x90, 0x00, 7, 7, 7, 7]) };

    function openWordWithAudio(rank: number) {
      cy.openWord(rank);
      cy.get(`${word} input[type="file"]`).selectFile(mine, { force: true });
      cy.get(inGroup(word, "Remove")).should("be.enabled");
    }

    it("plays a clip when it is uploaded, shows it playing and disabled, and queues the next clip behind it", () => {
      cy.visit("/open?rank=60", {
        onBeforeLoad(win) {
          const sounds: { src: string; onended: (() => void) | null }[] = [];
          class Stub {
            onended: (() => void) | null = null;
            onerror: (() => void) | null = null;
            constructor(readonly src: string) {
              sounds.push(this);
            }
            play() {
              return Promise.resolve();
            }
          }
          Object.assign(win, { Audio: Stub, sounds });
        },
      });
      cy.hydrated(by("Hint English"));
      const sounds = () => cy.window().its("sounds");
      cy.get(`${word} input[type="file"]`).selectFile(mine, { force: true });
      sounds().should("have.length", 1);
      cy.get(inGroup(word, "Playing the word")).should("be.disabled");
      cy.get(inGroup(word, "Play the word")).should("not.exist");
      cy.get(`${word} input[type="file"]`).selectFile({ ...mine, contents: Cypress.Buffer.from([0xff, 0xfb, 0x90, 0x00, 8, 8, 8, 8]) }, { force: true });
      cy.get(inGroup(word, "Upload")).should("be.enabled");
      sounds().should("have.length", 1);
      cy.window().then((win) => (win as unknown as { sounds: { onended: () => void }[] }).sounds[0].onended());
      sounds().should("have.length", 2);
      cy.window().then((win) => (win as unknown as { sounds: { onended: () => void }[] }).sounds[1].onended());
      cy.get(inGroup(word, "Play the word")).should("be.enabled");
    });

    it("keeps the audio actions in a capsule beside the field, with no menu, and the example capsule and its English field there from the start", () => {
      cy.openWord(40);
      cy.get(inGroup(word, "Play the word")).should("be.disabled");
      cy.get(inGroup(word, "Generate")).should("be.enabled");
      cy.get(inGroup(word, "Upload")).should("be.enabled");
      cy.get(inGroup(word, "Remove")).should("be.disabled");
      cy.get(`${word} [popover]`).should("not.exist");
      cy.get('[aria-label^="Audio options"]').should("not.exist");
      cy.get(inGroup(example, "New example")).should("be.enabled");
      cy.get(inGroup(word, "New example")).should("not.exist");
      cy.get(by("Example in English")).should("be.visible");
      cy.get(example).should("be.visible");
      cy.get(inGroup(example, "Generate")).should("be.disabled");
      cy.get(inGroup(example, "Upload")).should("be.disabled");
      cy.get(`${example} span[aria-hidden="true"]`).then(($line) => {
        const line = $line[0].getBoundingClientRect();
        cy.get(inGroup(example, "Remove")).then(($remove) => expect($remove[0].getBoundingClientRect().bottom).to.be.at.most(line.top));
        cy.get(inGroup(example, "New example")).then(($next) => expect($next[0].getBoundingClientRect().top).to.be.at.least(line.bottom));
      });
      cy.get(`${word} span[aria-hidden="true"]`).should("not.exist");
    });

    it("shows whether there is audio in the border: dashed without, solid with, amber when out of date", () => {
      cy.openWord(41);
      const border = () => cy.get(word).then(($group) => {
        const style = getComputedStyle($group[0]);
        return [style.borderTopStyle, style.borderTopColor];
      });
      border().should("deep.equal", ["dashed", "rgb(140, 138, 134)"]);
      cy.get(`${word} input[type="file"]`).selectFile(mine, { force: true });
      cy.get(inGroup(word, "Remove")).should("be.enabled");
      border().should("deep.equal", ["solid", "rgb(140, 138, 134)"]);
      cy.get(by("Arabic headword")).clear().type("قَلَمٌ").blur();
      cy.get(status).should("contain", "Saved");
      border().should("deep.equal", ["solid", "rgb(154, 91, 0)"]);
      cy.get(inGroup(word, "Regenerate (the Arabic changed)")).should("be.enabled");
    });

    for (const [width, height] of [[1280, 800], [390, 844]]) {
      it(`keeps the text centred and the pills of 24px buttons, as ${width >= 640 ? "a column at the right of" : "a row above"} the field, at ${width}px`, () => {
        cy.viewport(width, height);
        cy.openWord(42);
        cy.location("pathname").then((path) => {
          cy.request("PUT", `/api/notes/${path.split("/").pop()}`, { example: "الْكِتَابُ [فِي] الْحَقِيبَةِ", exampleEn: "The book is in the bag." });
          cy.reload();
          cy.hydrated(by("Hint English"));
        });
        cy.document().should((doc) => {
          const rect = (selector: string) => doc.querySelector(selector)!.getBoundingClientRect();
          const middle = (selector: string) => rect(selector).left + rect(selector).width / 2;
          const meaning = middle(by("English meaning"));
          for (const field of [by("Arabic headword"), by("Hint English"), by("Example in English")]) {
            expect(Math.abs(middle(field) - meaning), `${field} against the meaning`).to.be.at.most(1);
          }
          const fields = [[word, by("Arabic headword")], [example, `div${by("Example")}`]];
          for (const [group, field] of fields) {
            const buttons = Array.from(doc.querySelectorAll(`${group} button`)).map((button) => button.getBoundingClientRect());
            const axis = width >= 640 ? "left" : "top";
            expect(new Set(buttons.map((box) => Math.round(box[axis]))).size, `${group} shares one ${axis}`).to.equal(1);
            buttons.forEach((box) => {
              expect(box.height, `${group} button height`).to.be.at.least(24);
              expect(box.width, `${group} button width`).to.be.at.least(width >= 1024 ? 32 : 24);
            });
            if (width >= 640) expect(rect(group).left, `${group} right of its field`).to.be.at.least(rect(field).right - 1);
            else expect(rect(group).bottom, `${group} above its field`).to.be.at.most(rect(field).top);
          }
        });
      });
    }

    it("uploads, regenerates and removes", () => {
      openWordWithAudio(43);
      cy.get(inGroup(word, "Generate")).should("not.exist");
      cy.get(inGroup(word, "Regenerate")).click();
      cy.contains("There are no Google credentials");
      cy.get(inGroup(word, "Play the word")).should("be.enabled");
      cy.get(bar).should("not.contain", "audio for the word");
      cy.get(inGroup(word, "Remove")).click();
      cy.get(bar).should("contain", "Still needs").and("contain", "audio for the word");
      cy.get(inGroup(word, "Play the word")).should("be.disabled");
      cy.get(inGroup(word, "Generate"));
    });

    it("shows a spinner on the play button while it works, and moves nothing", () => {
      cy.intercept("POST", "/api/tts", (req) => req.reply({ delay: 800, statusCode: 400, body: { error: "Busy." } }));
      openWordWithAudio(44);
      cy.get(word).then(($group) => {
        const before = at($group[0]);
        cy.get(inGroup(word, "Regenerate")).click();
        cy.get(inGroup(word, "Play the word")).should("have.attr", "aria-busy", "true");
        cy.get(`${inGroup(word, "Play the word")} path[d="M7 4l13 8-13 8z"]`).should("not.exist");
        cy.get(inGroup(word, "Regenerate")).should("be.disabled");
        cy.get(word).should(($now) => expect(at($now[0])).to.deep.equal(before));
        cy.get(inGroup(word, "Play the word")).should("not.have.attr", "aria-busy");
        cy.get(inGroup(word, "Regenerate")).should("be.enabled");
      });
    });

    it("keeps a failed clip under its text, moving nothing, and lets you try again", () => {
      cy.openWord(45);
      cy.get(by("Hint English")).then(($hint) => {
        const before = at($hint[0])[1];
        cy.get(inGroup(word, "Generate")).click();
        cy.contains("There are no Google credentials");
        cy.get(by("Hint English")).should(($now) => expect(at($now[0])[1]).to.equal(before));
      });
      cy.get(inGroup(word, "Generate")).should("be.enabled");
    });

    it("moves nothing when the first example arrives", () => {
      cy.openWord(54);
      const watched = [by("Example in English"), example, `div${by("Example")}`];
      cy.document().then((doc) => {
        const before = watched.map((label) => at(doc.querySelector(label)!));
        cy.location("pathname").then((path) => {
          cy.request("PUT", `/api/notes/${path.split("/").pop()}`, { example: "الْكِتَابُ [فِي] الْحَقِيبَةِ", exampleEn: "The book is in the bag." }).then((response) => {
            cy.intercept("POST", "/api/mistral", { delay: 300, body: { note: response.body.note } });
          });
        });
        askForExample();
        cy.get(`${by("Example")} mark`).should("have.text", "فِي");
        cy.get(by("Example in English")).should("have.value", "The book is in the bag.");
        cy.document().should((now) => {
          watched.forEach((label, k) => {
            const after = at(now.querySelector(label)!);
            expect(Math.abs(after[0] - before[k][0]), `${label} left`).to.be.at.most(1);
            expect(Math.abs(after[1] - before[k][1]), `${label} top`).to.be.at.most(1);
            expect(Math.abs(after[3] - before[k][3]), `${label} height`).to.be.at.most(1);
          });
        });
      });
    });
  });

  it("keeps the lexical fields in the dialog, out of the card page, and Reset word out of the dialog", () => {
    cy.openWord(21);
    cy.get("details").should("not.exist");
    cy.contains("label", "Part of speech").should("not.be.visible");
    cy.get(tagsButton).click();
    cy.get("dialog[open]").contains("label", "Part of speech").should("be.visible");
    cy.get("dialog[open]").should("not.contain", "Reset word");
    cy.get(`dialog[open] ${resetButton}`).should("not.exist");
    cy.contains("label", "Gender");
    cy.contains("label", "Root");
    cy.contains("label", "Verb form");
    cy.contains("label", "Part of speech").find("select").select("adverb");
    cy.get(status).should("contain", "Saved");
  });

  it("says what the card still needs, in the action bar", () => {
    cy.openWord(22);
    cy.get(bar).should("contain", "Still needs an example, audio for the word");
    cy.get(`${word} input[type="file"]`).selectFile(unplayable, { force: true });
    cy.get(bar).should("contain", "Still needs an example").and("not.contain", "audio for the word");
  });

  it("approves the card and goes on to the next word", () => {
    cy.openWord(60);
    cy.location("pathname").then((card) => {
      cy.contains("button", /^Approve$/).click();
      cy.location("pathname").should("not.eq", card).and("match", /^\/cards\/[0-9a-f-]{36}$/);
      cy.visit(card);
      cy.get(bar).should("contain", "approved");
      cy.contains("button", /^Approve$/).should("not.exist");
      cy.contains("a", "Next").should("have.attr", "href", "/next");
    });
  });

  it("stays on the card and says why when it cannot be approved", () => {
    cy.openWord(61);
    cy.location("pathname").then((card) => {
      cy.get(by("English meaning")).clear().blur();
      cy.contains("button", /^Approve$/).click();
      cy.get(alert).should("contain", "The English meaning is empty.");
      cy.location("pathname").should("eq", card);
    });
  });

  it("shows a friendly page for a card that does not exist", () => {
    cy.visit("/cards/nope", { failOnStatusCode: false });
    cy.contains("There is nothing at this address.");
    cy.contains("a", "Back to the word list").should("have.attr", "href", "/");
  });
});

describe("resetting a word", () => {
  const editByMistake = (rank: number) => {
    cy.openWord(rank);
    cy.get(by("Hint English")).type("by mistake").blur();
    cy.contains(status, "Saved");
  };
  const askToReset = () => {
    cy.get(resetButton).click();
    cy.get("dialog[open]").should("contain", "Reset this word?");
  };

  it("deletes the card, goes back to the list and gives the word back as not started", () => {
    editByMistake(205);
    cy.location("pathname").then((card) => {
      askToReset();
      cy.get("dialog[open]").contains("button", "Reset").should("have.css", "background-color", "rgb(193, 63, 58)");
      cy.get("dialog[open]").contains("button", "Cancel").should("have.css", "background-color", "rgba(0, 0, 0, 0)");
      cy.axe("dialog[open] .action-danger");
      cy.get("dialog[open]").should("contain", "Not started").contains("button", "Reset").click();
      cy.location("pathname").should("eq", "/");
      cy.visit(`/?q=${encodeURIComponent("rank:205")}`);
      cy.wordRow(205).should("not.contain", "in progress");
      cy.wordRow(205).find("a").should("have.attr", "href").and("include", "/open?rank=205");
      cy.request({ url: card, failOnStatusCode: false }).its("status").should("eq", 404);
    });
  });

  it("keeps the card when you change your mind, and gives the focus back to the Reset word button", () => {
    editByMistake(206);
    cy.location("pathname").then((card) => {
      askToReset();
      cy.get("dialog[open]").contains("button", "Cancel").click();
      cy.get("dialog[open]").should("not.exist");
      cy.focused().should("have.attr", "title", "Reset word");
      cy.location("pathname").should("eq", card);
      cy.get(by("Hint English")).should("have.value", "by mistake");
    });
  });

  it("goes back to the list the card was opened from", () => {
    const query = "rank:207-208";
    cy.visit(`/?q=${encodeURIComponent(query)}`);
    cy.wordRow(207).click();
    cy.hydrated(by("Hint English"));
    askToReset();
    cy.get("dialog[open]").contains("button", "Reset").click();
    cy.location("search").should("eq", `?q=${encodeURIComponent(query)}`);
    cy.wordRow(207).should("exist");
  });

  it("shows the red confirmation with readable text in dark mode too", () => {
    editByMistake(212);
    cy.colorScheme("dark");
    askToReset();
    cy.get("dialog[open]").contains("button", "Reset")
      .should("have.css", "background-color", "rgb(229, 115, 115)")
      .and("have.css", "color", "rgb(25, 25, 25)");
    cy.axe("dialog[open] .action-danger");
  });

  it("puts Reset word right of the word counter", () => {
    cy.openWord(209);
    cy.get(resetButton).should("not.have.attr", "aria-label");
    cy.contains("span", "Word 209 of 5000").then(($counter) => {
      cy.get(resetButton).should("be.visible").then(($button) => {
        const counter = $counter[0].getBoundingClientRect();
        const button = $button[0].getBoundingClientRect();
        expect(button.left).to.be.at.least(counter.right);
        expect(Math.abs((button.top + button.bottom) / 2 - (counter.top + counter.bottom) / 2)).to.be.at.most(8);
      });
    });
    cy.axe(resetButton);
  });
});
