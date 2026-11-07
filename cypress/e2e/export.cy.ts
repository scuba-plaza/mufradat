export {};

const searchBar = 'form[role="search"] input';
const searchButton = 'form[role="search"] button';
const exportButton = 'button[title^="Export "]';
const status = '[role="search"] ~ [role="status"]';
const notice = '[role="search"] ~ [popover][role="alert"]';
const tag = "zz_export";
const deckHeaders = {
  "Content-Type": "application/octet-stream",
  "Content-Disposition": 'attachment; filename="arabic-deck-2026-01-01.apkg"',
};
const tagged: string[] = [];
const title = (count: number) => `Export ${count} approved ${count === 1 ? "card" : "cards"}`;

function approvedCard(rank: number, tags: string[] = []) {
  cy.visit(`/open?rank=${rank}`);
  cy.location("pathname").then((path) => {
    const id = path.split("/").pop()!;
    tagged.push(id);
    cy.request("POST", `/api/notes/${id}/approve`, {});
    cy.request("PUT", `/api/notes/${id}/tags`, { tags });
  });
}

function visit(path: string) {
  cy.visit(path);
  cy.hydrated(exportButton);
}

describe("the export icon", () => {
  afterEach(() => {
    cy.colorScheme("light");
    tagged.splice(0).forEach((id) => cy.request("PUT", `/api/notes/${id}/tags`, { tags: [] }));
  });

  it("sits right of Search, is named by how many approved cards the search holds, and exports exactly those", () => {
    approvedCard(4405);
    approvedCard(4401, [tag]);
    approvedCard(4402, [tag]);
    visit(`/?q=${encodeURIComponent(`tag:${tag}`)}`);
    cy.get(exportButton).should("have.attr", "title", title(2));
    cy.get(exportButton).should("not.have.attr", "aria-label");
    cy.get(exportButton).should("not.have.attr", "role");
    cy.get(`${exportButton} svg`).should("have.attr", "aria-hidden", "true");
    cy.get(`${exportButton} svg`).should("have.attr", "focusable", "false");
    cy.get(searchBar).should("exist");
    cy.get(searchButton).then(($search) => {
      cy.get(exportButton).then(($icon) => {
        const search = $search[0].getBoundingClientRect();
        const icon = $icon[0].getBoundingClientRect();
        expect(icon.left).to.be.at.least(search.right);
        expect(Math.abs((icon.top + icon.bottom) / 2 - (search.top + search.bottom) / 2)).to.be.at.most(2);
      });
    });
    cy.task<string[]>("downloads").then((before) => {
      cy.intercept("GET", "/api/export?q=*").as("export");
      cy.get(exportButton).click();
      cy.wait("@export").its("request.url").should("include", `q=${encodeURIComponent(`tag:${tag}`)}`);
      cy.get("@export.all").should("have.length", 1);
      cy.task<string[]>("downloads", before.length + 1).should((after) => expect(after).to.have.length(before.length + 1));
      cy.task<string[]>("workspaceExports").should("deep.equal", []);
      cy.get(status).should("have.text", "");
    });
  });

  it("counts every approved card for an empty search and asks for the whole deck", () => {
    approvedCard(4403);
    visit("/");
    cy.request("/api/backups").its("body.check.approved").then((approved: number) => {
      cy.get(exportButton).should("have.attr", "title", title(approved));
    });
    cy.intercept("GET", "/api/export*").as("export");
    cy.get(exportButton).click();
    cy.wait("@export").its("request.url").should("not.include", "q=");
  });

  it("is disabled, titled Export 0 approved cards, when the search holds none", () => {
    visit(`/?q=${encodeURIComponent("tag:none_such")}`);
    cy.get(exportButton).should("be.disabled");
    cy.get(exportButton).should("have.attr", "title", "Export 0 approved cards");
  });

  it("ignores text typed in the bar until it is searched", () => {
    approvedCard(4404, [tag]);
    visit(`/?q=${encodeURIComponent(`tag:${tag}`)}`);
    cy.get(exportButton).invoke("attr", "title").then((before) => {
      cy.get(searchBar).clear().type("tag:other");
      cy.get(exportButton).should("have.attr", "title", before);
      cy.intercept("GET", "/api/export*", { headers: deckHeaders, body: "x" }).as("export");
      cy.get(exportButton).click();
      cy.wait("@export").its("request.url").should("include", `q=${encodeURIComponent(`tag:${tag}`)}`);
    });
  });

  it("shows a spinner in place of the icon while the deck is built, and moves nothing", () => {
    approvedCard(4403);
    visit("/");
    cy.intercept("GET", "/api/export*", { delay: 600, headers: deckHeaders, body: "x" });
    cy.get(exportButton).then(($icon) => {
      cy.get(searchBar).then(($input) => {
        const page = $icon[0].ownerDocument;
        const place = (el: Element) => {
          const box = el.getBoundingClientRect();
          return [box.left, box.top + page.defaultView!.scrollY, box.width, box.height];
        };
        const before = [$icon[0], $input[0], page.querySelector("main")!].map(place);
        cy.wrap($icon).click();
        cy.get(exportButton).should("have.attr", "aria-busy", "true");
        cy.get(`${exportButton} path[d="M12 15V3"]`).should("not.exist");
        cy.get(exportButton).should(($now) => {
          const after = [$now[0], page.querySelector(searchBar)!, page.querySelector("main")!].map(place);
          expect(after).to.deep.equal(before);
        });
        cy.get(status).then(($line) => expect($line[0].getBoundingClientRect().height).to.be.at.most(1));
        cy.get(status).should("have.text", "Preparing the deck…");
        cy.get(exportButton).should("not.have.attr", "aria-busy");
        cy.get(`${exportButton} path[d="M12 15V3"]`).should("exist");
        cy.get(status).should("have.text", "");
      });
    });
  });

  it("asks for the deck once however often it is clicked while it is built", () => {
    approvedCard(4403);
    visit("/");
    cy.intercept("GET", "/api/export*", { delay: 600, headers: deckHeaders, body: "x" }).as("export");
    cy.get(exportButton).dblclick();
    cy.get(status).should("have.text", "Preparing the deck…");
    cy.get(status).should("have.text", "");
    cy.get("@export.all").should("have.length", 1);
  });

  it("says why when the export is refused and saves nothing", () => {
    approvedCard(4403);
    visit("/");
    cy.task<string[]>("downloads").then((before) => {
      cy.intercept("GET", "/api/export*", { statusCode: 400, body: { error: "The card of the word X has no audio file." } });
      cy.get(exportButton).click();
      cy.get(notice).should("be.visible").and("have.text", "The card of the word X has no audio file.");
      cy.task<string[]>("downloads").should("deep.equal", before);
    });
  });

  it("keeps the tabs where they are when the export is refused", () => {
    approvedCard(4403);
    visit("/");
    cy.intercept("GET", "/api/export*", { statusCode: 400, body: { error: "The card of the word X has no audio file." } });
    const placed = ($tabs: JQuery<HTMLElement>) => $tabs[0].getBoundingClientRect().top + $tabs[0].ownerDocument.defaultView!.scrollY;
    cy.get('nav[aria-label="Status"]').then(($tabs) => {
      const before = placed($tabs);
      cy.get(exportButton).click();
      cy.get(notice).should("be.visible").and("have.text", "The card of the word X has no audio file.");
      cy.get('nav[aria-label="Status"]').should(($now) => expect(placed($now)).to.equal(before));
    });
  });

  it("closes the message with Escape", () => {
    approvedCard(4403);
    visit("/");
    cy.intercept("GET", "/api/export*", { statusCode: 400, body: { error: "The card of the word X has no audio file." } });
    cy.get(exportButton).click();
    cy.get(notice).should("be.visible");
    cy.pressEscape();
    cy.get(notice).should("not.be.visible");
  });

  it("says Request failed (500). when the server answers with something that is not JSON", () => {
    approvedCard(4403);
    visit("/");
    cy.intercept("GET", "/api/export*", { statusCode: 500, headers: { "Content-Type": "text/html" }, body: "<!DOCTYPE html><p>oops</p>" });
    cy.get(exportButton).click();
    cy.get(notice).should("be.visible").and("have.text", "Request failed (500).");
  });

  it("says Request failed (400). when the refusal carries no message", () => {
    approvedCard(4403);
    visit("/");
    cy.intercept("GET", "/api/export*", { statusCode: 400, body: {} });
    cy.get(exportButton).click();
    cy.get(notice).should("be.visible").and("have.text", "Request failed (400).");
  });

  it("keeps the icon in the row and the message inside the window, and does not scroll sideways, at 390px", () => {
    cy.viewport(390, 844);
    approvedCard(4403);
    visit("/");
    cy.intercept("GET", "/api/export*", { statusCode: 400, body: { error: "The audio for كِتَابٌ (word) is missing from the workspace. ".repeat(4) } });
    cy.get(exportButton).click();
    cy.get(notice).should("be.visible").and("contain", "is missing from the workspace");
    cy.get(searchButton).then(($search) => {
      cy.get(exportButton).then(($icon) => {
        cy.get(notice).then(($message) => {
          const search = $search[0].getBoundingClientRect();
          const icon = $icon[0].getBoundingClientRect();
          const box = $message[0].getBoundingClientRect();
          expect(Math.abs((icon.top + icon.bottom) / 2 - (search.top + search.bottom) / 2)).to.be.at.most(2);
          expect(box.left).to.be.at.least(0);
          expect(box.right).to.be.at.most($message[0].ownerDocument.defaultView!.innerWidth);
        });
      });
    });
    cy.document().should((doc) => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
  });

  it("has no accessibility problems on the home page with the icon, in light and dark", () => {
    approvedCard(4403);
    visit("/");
    cy.axe();
    cy.colorScheme("dark");
    cy.axe();
  });
});
