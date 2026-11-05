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
      cy.get(by("English meaning")).focus();
      cy.press(Cypress.Keyboard.Keys.TAB);
      cy.focused().should("contain", "Edit forms").should(($field) => {
        let fixed = $field[0].ownerDocument.querySelector(bar) as HTMLElement;
        while (getComputedStyle(fixed).position !== "fixed") fixed = fixed.parentElement!;
        expect($field[0].getBoundingClientRect().bottom).to.be.at.most(fixed.getBoundingClientRect().top);
      });
    });
  });

  describe("the action bar", () => {
    let rank = 80;

    for (const [width, height] of screens) {
      it(`gives Skip, Approve and Next the same size at ${width}px`, () => {
        cy.viewport(width, height);
        openWord(rank++);
        cy.contains(`${bar} button`, "Skip").then(($skip) => {
          cy.contains(`${bar} button`, "Approve and next").then(($approve) => {
            const pair = sizeOf($skip[0]);
            expect(sizeOf($approve[0]), "Approve against Skip").to.deep.equal(pair);
            if (width < 640) {
              const text = $skip[0].ownerDocument.querySelector(`${bar} p`)!;
              expect(text.getBoundingClientRect().bottom, "the status text above the buttons").to.be.at.most($skip[0].getBoundingClientRect().top);
            }
            cy.location("pathname").then((card) => {
              cy.wrap($approve).click();
              cy.location("pathname").should("not.eq", card);
              cy.visit(card);
              cy.contains(`${bar} a`, "Next").should(($next) => expect(sizeOf($next[0]), "Next against the pair").to.deep.equal(pair));
            });
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
            const gap = form.getBoundingClientRect().top - label.getBoundingClientRect().bottom;
            expect(gap, `gap between the label ${label.textContent} and its Arabic`).to.be.at.least(minimumGap);
          });
        });
      });
    }
  });

  describe("the headword and its audio", () => {
    const minimumGap = 8;
    let rank = 70;

    for (const [width, height] of screens) {
      for (const withAudio of [false, true]) {
        it(`leave a gap of ${minimumGap}px between them at ${width}px ${withAudio ? "with" : "without"} audio`, () => {
          cy.viewport(width, height);
          openWord(rank++);
          if (withAudio) {
            cy.get(`${by("Audio for the word")} input[type="file"]`).selectFile(unplayable, { force: true });
            cy.get(by("Play the word"));
          }
          cy.get(by("Arabic headword")).should(($field) => {
            const control = $field[0].ownerDocument.querySelector(`${by("Audio for the word")} button`)!;
            const gap = control.getBoundingClientRect().top - $field[0].getBoundingClientRect().bottom;
            expect(gap, "gap between the headword and the first audio control").to.be.at.least(minimumGap);
          });
        });
      }
    }
  });
});
