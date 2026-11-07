export {};

const list = '[aria-label="Word list"]';
const kitab = encodeURIComponent("كتاب");

/** Passes once the font that the element asks for first has been loaded by the page. */
function fontLoaded($el: JQuery<HTMLElement>) {
  const asked = getComputedStyle($el[0]).fontFamily.split(",")[0].replace(/["']/g, "").trim();
  const doc = $el[0].ownerDocument;
  return doc.fonts.ready.then(() => {
    const loaded: string[] = [];
    doc.fonts.forEach((face) => face.status === "loaded" && loaded.push(face.family.replace(/["']/g, "")));
    expect(loaded).to.include(asked);
  });
}

describe("the look of the pages", () => {
  it("keeps list rows compact, so a page of words is quick to scan", () => {
    cy.visit("/");
    cy.get(`${list} > li`).should("have.length", 30);
    cy.get(list).invoke("outerHeight").should("be.lessThan", 1440);
  });

  it("keeps the card's actions at the bottom of the window while the page scrolls", () => {
    cy.viewport(1280, 360);
    cy.visit("/open?rank=65");
    cy.get('[role="toolbar"]').should(($bar) => {
      const { bottom } = $bar[0].getBoundingClientRect();
      expect(bottom).to.be.within(359, 360);
    });
  });

  it("does not scroll sideways on a phone", () => {
    cy.viewport(375, 700);
    for (const path of ["/", `/?q=${kitab}`, "/open?rank=3"]) {
      cy.visit(path);
      cy.document().should((doc) => expect(doc.documentElement.scrollWidth, path).to.be.at.most(375));
    }
  });

  it("loads the Arabic font that the page asks for", () => {
    cy.visit("/");
    cy.get(`${list} [lang="ar"]`).first().then(fontLoaded);
  });
});
