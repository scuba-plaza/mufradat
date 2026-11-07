export {};

const by = (label: string) => `[aria-label="${label}"]`;
const tagsButton = '[title="Tags and details"]';
const addTag = by("Add tag");
const chips = "dialog[open] ul li";

function openWord(rank: number) {
  cy.visit(`/open?rank=${rank}`);
  cy.hydrated(by("Hint"));
}

function setTags(tags: string[]) {
  cy.location("pathname").then((path) => cy.request("PUT", `/api/notes/${path.split("/").pop()}/tags`, { tags }));
}

function reopen() {
  cy.reload();
  cy.hydrated(by("Hint"));
  cy.get(tagsButton).click();
}

describe("the tags and details of a card", () => {
  afterEach(() => cy.colorScheme("light"));

  it("has one icon button for tags and details, right of Reset word", () => {
    openWord(4201);
    cy.get(tagsButton).should("be.visible").and("not.have.attr", "aria-label");
    cy.get('[title="Reset word"]').then(($reset) => {
      cy.get(tagsButton).then(($tags) => {
        const reset = $reset[0].getBoundingClientRect();
        const tags = $tags[0].getBoundingClientRect();
        expect(tags.left).to.be.at.least(reset.right);
        expect(Math.abs((tags.top + tags.bottom) / 2 - (reset.top + reset.bottom) / 2)).to.be.at.most(2);
      });
    });
    cy.get(tagsButton).click();
    cy.get("dialog[open]").should("be.visible").and("contain", "Tags and details of ");
  });

  it("focuses Add tag when the dialog opens, and gives the focus back to the button on Esc", () => {
    openWord(4202);
    cy.get(tagsButton).click();
    cy.focused().should("have.attr", "aria-label", "Add tag");
    cy.pressEscape();
    cy.get("dialog[open]").should("not.exist");
    cy.focused().should("have.attr", "title", "Tags and details");
  });

  it("adds a tag with Enter, shows it as stored and keeps it after a reload", () => {
    openWord(4203);
    cy.get(tagsButton).click();
    cy.get(addTag).type("Lesson 1{enter}");
    cy.get(chips).should("have.length", 1).and("contain", "lesson_1");
    cy.get(addTag).should("have.value", "");
    reopen();
    cy.get(chips).should("have.length", 1).and("contain", "lesson_1");
    setTags([]);
  });

  it("removes a tag with its button", () => {
    openWord(4204);
    setTags(["lesson_1", "grammar"]);
    reopen();
    cy.get(by("Remove tag lesson_1")).click();
    cy.get(chips).should("have.length", 1).and("contain", "grammar");
    reopen();
    cy.get(chips).should("have.length", 1).and("contain", "grammar");
    setTags([]);
  });

  it("leaves an approved card approved", () => {
    openWord(4205);
    cy.location("pathname").then((path) => cy.request("POST", `/api${path.replace("/cards", "/notes")}/approve`, {}));
    cy.request("/api/backups").its("body.check.approved").then((before) => {
      cy.reload();
      cy.hydrated(by("Hint"));
      cy.get(tagsButton).click();
      cy.get(addTag).type("lesson_1{enter}");
      cy.get(chips).should("contain", "lesson_1");
      cy.get('[role="status"]').should("contain", "Saved");
      cy.request("/api/backups").its("body.check.approved").should("eq", before);
    });
    cy.pressEscape();
    cy.get('[role="toolbar"]').contains("a", "Next");
    setTags([]);
  });

  it("says why a tag is refused, inside the dialog, and keeps the tags it had", () => {
    openWord(4206);
    setTags(["kept"]);
    reopen();
    cy.get(addTag).type("a,b{enter}");
    cy.get('dialog[open] [role="alert"]').should("contain", "A tag cannot contain commas, quotes or *.");
    cy.get(chips).should("have.length", 1).and("contain", "kept");
    setTags([]);
  });

  it("has no accessibility problems in light and dark with the dialog open", () => {
    openWord(4207);
    setTags(["lesson_1"]);
    reopen();
    cy.get("dialog[open]").should("be.visible");
    cy.axe("dialog[open]");
    cy.colorScheme("dark");
    cy.axe("dialog[open]");
    setTags([]);
  });
});
