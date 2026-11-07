export {};

const by = (label: string) => `[aria-label="${label}"]`;
const unplayable = { contents: Cypress.Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]), fileName: "word.mp3", mimeType: "audio/mpeg" };
const screens = [[1920, 1100], [1280, 800], [390, 844]];

function openWord(rank: number) {
  cy.visit(`/open?rank=${rank}`);
  cy.hydrated(by("Hint"));
}

const bar = '[role="toolbar"]';
const sizeOf = (element: HTMLElement) => {
  const { width, height } = element.getBoundingClientRect();
  return { width: width.toFixed(1), height: height.toFixed(1) };
};

describe("the layout of a card", () => {
  describe("keyboard focus", () => {
    it("keeps the control that Tab reaches clear of the action bar in a short window", () => {
      cy.viewport(1280, 400);
      openWord(100);
      cy.get(by("Example in English")).focus();
      cy.press(Cypress.Keyboard.Keys.TAB);
      cy.focused().should("have.attr", "title", "New example").should(($field) => {
        let fixed = $field[0].ownerDocument.querySelector(bar) as HTMLElement;
        while (getComputedStyle(fixed).position !== "fixed") fixed = fixed.parentElement!;
        expect($field[0].getBoundingClientRect().bottom).to.be.at.most(fixed.getBoundingClientRect().top);
      });
    });
  });

  describe("the height of a card", () => {
    for (const [width, height] of [[1920, 1200], [1280, 800]]) {
      it(`is no taller than 880px for a word with forms, an example and a translation at ${width}px`, () => {
        cy.viewport(width, height);
        openWord(6);
        cy.location("pathname").then((path) => {
          cy.request("PUT", `/api/notes/${path.split("/").pop()}`, { example: "لَدَيْهِ سَيَّارَةٌ [خَاصَّةٌ]", exampleEn: "He has a private car." });
          cy.visit(path);
          cy.hydrated(by("Hint"));
        });
        cy.get("main").should(($main) => expect($main[0].getBoundingClientRect().height).to.be.at.most(880));
      });
    }
  });

  describe("the order of a card", () => {
    const cards = [
      { rank: 6, what: "forms" },
      { rank: 3, what: "no forms" },
    ];

    for (const [width, height] of [[1920, 1100], [390, 844]]) {
      for (const { rank, what } of cards) {
        it(`shows the headword, the hint, the forms, the English meaning and the example in that order at ${width}px with ${what}`, () => {
          cy.viewport(width, height);
          openWord(rank);
          const order = [by("Arabic headword"), by("Hint"), by("Forms"), by("English meaning"), `div${by("Example")}`];
          cy.document().should((doc) => {
            const boxes = order.map((selector) => {
              const element = doc.querySelector(selector);
              expect(element, selector).to.not.equal(null);
              return element!;
            });
            boxes.slice(1).forEach((element, i) => {
              const before = boxes[i];
              expect(element.getBoundingClientRect().top, `${order[i + 1]} below ${order[i]}`).to.be.at.least(before.getBoundingClientRect().bottom);
              expect(before.compareDocumentPosition(element) & doc.defaultView!.Node.DOCUMENT_POSITION_FOLLOWING, `${order[i + 1]} after ${order[i]} for Tab`).to.not.equal(0);
            });
          });
        });
      }
    }
  });

  describe("the space under a text field", () => {
    const minimumGap = 8;
    const byText = (doc: Document, text: string) =>
      Array.from(doc.querySelectorAll("button")).find((button) => button.textContent?.trim() === text) ?? null;
    const example = `div${by("Example")}`;
    const pairs: {
      what: string;
      rank: number;
      written?: boolean;
      above: (doc: Document) => Element | null;
      below: (doc: Document) => Element | null;
    }[] = [
      { what: "the hint and the empty forms", rank: 7, above: (doc) => doc.querySelector(by("Hint")), below: (doc) => doc.querySelector(by("Forms")) },
      { what: "the hint and the forms", rank: 6, above: (doc) => doc.querySelector(by("Hint")), below: (doc) => doc.querySelector(by("Forms")) },
      { what: "the English meaning and the example", rank: 7, above: (doc) => doc.querySelector(by("English meaning")), below: (doc) => doc.querySelector(`${example} button`) },
      { what: "an empty example and the English translation", rank: 7, above: (doc) => doc.querySelector(example), below: (doc) => doc.querySelector(by("Example in English")) },
      { what: "the example and its translation", rank: 110, written: true, above: (doc) => doc.querySelector(example), below: (doc) => doc.querySelector(by("Example in English")) },
    ];

    for (const [width, height] of [[1920, 1100], [390, 844]]) {
      for (const { what, rank, written, above, below } of pairs) {
        it(`leaves at least ${minimumGap}px between ${what} at ${width}px`, () => {
          cy.viewport(width, height);
          openWord(rank);
          if (written) {
            cy.location("pathname").then((path) => {
              cy.request("PUT", `/api/notes/${path.split("/").pop()}`, { example: "كَانَ [الْكِتَابُ] جَمِيلًا جِدًّا", exampleEn: "The book was very beautiful." });
              cy.visit(path);
              cy.hydrated(by("Hint"));
            });
          }
          cy.document().should((doc) => {
            const upper = above(doc);
            const lower = below(doc);
            expect(upper, `${what}: the upper element`).to.not.equal(null);
            expect(lower, `${what}: the lower element`).to.not.equal(null);
            expect(lower!.getBoundingClientRect().top - upper!.getBoundingClientRect().bottom, `gap between ${what}`).to.be.at.least(minimumGap);
          });
        });
      }
    }
  });

  describe("the action bar", () => {
    let rank = 80;

    for (const [width, height] of screens) {
      it(`gives Approve and Next the same size at ${width}px`, () => {
        cy.viewport(width, height);
        openWord(rank++);
        cy.contains(`${bar} button`, "Approve").then(($approve) => {
          const approve = sizeOf($approve[0]);
          if (width < 640) {
            const text = $approve[0].ownerDocument.querySelector(`${bar} p`)!;
            expect(text.getBoundingClientRect().bottom, "the status text above the button").to.be.at.most($approve[0].getBoundingClientRect().top);
          }
          cy.location("pathname").then((card) => {
            cy.wrap($approve).click();
            cy.location("pathname").should("not.eq", card);
            cy.visit(card);
            cy.contains(`${bar} a`, "Next").should(($next) => expect(sizeOf($next[0]), "Next against Approve").to.deep.equal(approve));
          });
        });
      });
    }
  });

  describe("the forms panel", () => {
    const minimumGap = 4;

    for (const [width, height] of [[1920, 1100], [390, 844]]) {
      it(`keeps each label clear of its Arabic at ${width}px`, () => {
        cy.viewport(width, height);
        openWord(6);
        cy.get(`${by("Forms")} .ar`).should(($forms) => {
          expect($forms.length).to.be.at.least(2);
          $forms.each((_, form) => {
            const label = form.previousElementSibling!;
            const gap = form.getBoundingClientRect().left - label.getBoundingClientRect().right;
            expect(gap, `gap between the label ${label.textContent} and its Arabic`).to.be.at.least(minimumGap);
          });
        });
      });
    }
  });
});
