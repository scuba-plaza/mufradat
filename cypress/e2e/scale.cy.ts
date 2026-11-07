export {};

const bar = '[role="toolbar"]';
const list = '[aria-label="Word list"]';
const mp3 = [0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4];
const kitab = encodeURIComponent("كتاب");
const columnRem = 58;

const px = (value: string) => parseFloat(value);

function openFullCard(rank: number) {
  cy.visit(`/open?rank=${rank}`);
  cy.location("pathname").then((path) => {
    const id = path.split("/").pop();
    cy.request("PUT", `/api/notes/${id}`, { example: "كَانَ [الْكِتَابُ] جَمِيلًا جِدًّا", exampleEn: "The book was very beautiful." }).then((response) => {
      const { forms } = (response.body as { note: { forms: { id: string }[] } }).note;
      cy.window().then(async (win) => {
        for (const slot of ["word", "example", ...forms.map((form) => form.id)]) {
          await win.fetch(`/api/audio/${id}/${slot}`, { method: "PUT", body: new win.Uint8Array(mp3) });
        }
      });
    });
    cy.visit(path);
    cy.hydrated('[aria-label="Hint"]');
  });
}

describe("the scale of the interface", () => {
  describe("on a large screen", () => {
    beforeEach(() => cy.viewport(1920, 1100));

    it("sets text, controls and the top bar at the larger sizes", () => {
      cy.visit("/");
      cy.get("body").should(($body) => expect(px($body.css("font-size"))).to.equal(18));
      cy.get('[aria-label="Next word"] + p, [aria-label="Next word"] p').last().should(($p) => expect(px($p.css("font-size"))).to.equal(16));
      cy.get(`${list} .num`).first().should(($rank) => expect(px($rank.css("font-size"))).to.equal(14));
      cy.contains("a", "Continue").should(($link) => expect($link[0].getBoundingClientRect().height).to.equal(44));
      cy.get("header .topbar-inner").should(($inner) => expect($inner[0].getBoundingClientRect().height).to.equal(56));
      cy.get(`${list} > li a`).first().should(($row) => expect($row[0].getBoundingClientRect().height).to.equal(44));
    });

    for (const [width, height] of [[1920, 1100], [2560, 1440]]) {
      it(`keeps one centered column of ${columnRem}rem on a ${width}px screen`, () => {
        cy.viewport(width, height);
        cy.visit("/");
        cy.get("main").should(($main) => {
          const { left, right, width: column } = $main[0].getBoundingClientRect();
          expect(column).to.equal(columnRem * 16);
          expect(Math.abs(left - ($main[0].ownerDocument.documentElement.clientWidth - right))).to.be.at.most(1);
        });
      });
    }

    it("fits a card with four forms, a marked example and its clips, with room to spare", () => {
      openFullCard(6);
      cy.get('[aria-label="Audio for the example"]').should(($button) => {
        const content = $button[0].parentElement!.parentElement!.getBoundingClientRect().bottom;
        const barTop = $button[0].ownerDocument.querySelector(bar)!.getBoundingClientRect().top;
        expect(barTop - content, "room between the last content and the bar").to.be.at.least(20);
      });
      cy.document().its("documentElement.scrollHeight").should("be.at.most", 1100);
    });

    it("shows the next word, Continue, the progress, the search bar and ten rows of the list without scrolling", () => {
      cy.visit("/");
      for (const selector of ['[aria-label="Next word"]', '[aria-label="Next word"] a', '[role="progressbar"]', 'form[role="search"]', `${list} > li:nth-child(10)`]) {
        cy.get(selector).should(($el) => expect($el[0].getBoundingClientRect().bottom, selector).to.be.at.most(1100));
      }
    });
  });

  describe("on a phone", () => {
    beforeEach(() => cy.viewport(390, 844));

    it("sets text and every field at 16px or more", () => {
      for (const path of ["/", `/?q=${kitab}`, "/open?rank=6"]) {
        cy.visit(path);
        cy.get("body").should(($body) => expect(px($body.css("font-size")), path).to.be.at.least(16));
        cy.get("body").then(($body) => {
          $body.find("input, select, textarea").each((_, element) => {
            const $field = Cypress.$(element);
            if ($field.attr("type") === "hidden" || $field.attr("type") === "file") return;
            expect(px($field.css("font-size")), `${path} ${$field.attr("aria-label") ?? $field.attr("name")}`).to.be.at.least(16);
          });
        });
      }
    });
  });
});
