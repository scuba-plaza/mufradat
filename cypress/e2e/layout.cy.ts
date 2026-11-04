export {};

const by = (label: string) => `[aria-label="${label}"]`;
const unplayable = { contents: Cypress.Buffer.from([0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4]), fileName: "word.mp3", mimeType: "audio/mpeg" };
const screens = [[1920, 1100], [1280, 800], [390, 844]];

function openWord(rank: number) {
  cy.visit(`/open?rank=${rank}`);
  cy.hydrated(by("Hint"));
}

describe("the layout of a card", () => {
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
