export {};

const by = (label: string) => `[aria-label="${label}"]`;
const tagsButton = '[title="Tags and details"]';
const addTag = by("Add tag");
const chips = "dialog[open] ul li";

const tagged: string[] = [];

function openWord(rank: number) {
  cy.openWord(rank);
  cy.location("pathname").then((path) => tagged.push(path.split("/").pop()!));
}

function setTags(tags: string[]) {
  cy.location("pathname").then((path) => cy.request("PUT", `/api/notes/${path.split("/").pop()}/tags`, { tags }));
}

function reopen() {
  cy.reload();
  cy.hydrated(by("Hint English"));
  cy.get(tagsButton).click();
}

describe("the tags and details of a card", () => {
  afterEach(() => {
    tagged.splice(0).forEach((id) => {
      cy.request("PUT", `/api/notes/${id}/tags`, { tags: [] });
      cy.request("PUT", `/api/notes/${id}`, {});
    });
  });

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

  it("adds a tag with Enter, shows it as stored without changing the height of the dialog, and keeps it after a reload", () => {
    openWord(4203);
    cy.get(tagsButton).click();
    cy.get("dialog[open]").then(($dialog) => {
      const before = $dialog[0].getBoundingClientRect().height;
      cy.get(addTag).type("Lesson 1{enter}");
      cy.get(chips).should("have.length", 1).and("contain", "lesson_1");
      cy.get("dialog[open]").should(($now) => expect($now[0].getBoundingClientRect().height).to.equal(before));
    });
    cy.get(addTag).should("have.value", "");
    reopen();
    cy.get(chips).should("have.length", 1).and("contain", "lesson_1");
  });

  it("removes a tag with its button", () => {
    openWord(4204);
    setTags(["lesson_1", "grammar"]);
    reopen();
    cy.get(by("Remove tag lesson_1")).click();
    cy.get(chips).should("have.length", 1).and("contain", "grammar");
    reopen();
    cy.get(chips).should("have.length", 1).and("contain", "grammar");
  });

  it("leaves an approved card approved", () => {
    openWord(4205);
    cy.location("pathname").then((path) => cy.request("POST", `/api${path.replace("/cards", "/notes")}/approve`, {}));
    cy.request("/api/backups").its("body.check.approved").then((before) => {
      cy.reload();
      cy.hydrated(by("Hint English"));
      cy.get(tagsButton).click();
      cy.get(addTag).type("lesson_1{enter}");
      cy.get(chips).should("contain", "lesson_1");
      cy.get('[role="status"]').should("contain", "Saved");
      cy.request("/api/backups").its("body.check.approved").should("eq", before);
    });
    cy.pressEscape();
    cy.get('[role="toolbar"]').contains("a", "Next");
  });

  it("says why a tag is refused, inside the dialog, keeps the tags it had and what was typed, and drops the refusal when you type again or reopen, at the same height", () => {
    openWord(4206);
    setTags(["kept"]);
    reopen();
    cy.get("dialog[open]").then(($dialog) => {
      const before = $dialog[0].getBoundingClientRect().height;
      cy.get(addTag).type("a,b{enter}");
      cy.get('dialog[open] [role="alert"]').should("contain", "A tag cannot contain commas, quotes or *.");
      cy.get("dialog[open]").should(($now) => expect($now[0].getBoundingClientRect().height).to.equal(before));
      cy.get(chips).should("have.length", 1).and("contain", "kept");
      cy.get(addTag).should("have.value", "a,b");
      cy.get(addTag).type("c");
      cy.get('dialog[open] [role="alert"]').should("not.exist");
      cy.get("dialog[open]").should(($now) => expect($now[0].getBoundingClientRect().height).to.equal(before));
    });
    cy.get(addTag).type("{enter}");
    cy.get('dialog[open] [role="alert"]').should("exist");
    cy.pressEscape();
    cy.get("dialog[open]").should("not.exist");
    cy.get(tagsButton).click();
    cy.get('dialog[open] [role="alert"]').should("not.exist");
    cy.get(addTag).should("have.value", "");
  });

  it("has no accessibility problems in light and dark with the dialog open", () => {
    openWord(4207);
    setTags(["lesson_1"]);
    reopen();
    cy.get("dialog[open]").should("be.visible");
    cy.axe("dialog[open]");
    cy.colorScheme("dark");
    cy.axe("dialog[open]");
  });

  it("says why a detail was refused, inside the dialog, and keeps the height of the dialog", () => {
    openWord(4208);
    cy.intercept("PUT", "/api/notes/*", { statusCode: 500, body: { error: "The disk is full." } });
    cy.get(tagsButton).click();
    cy.get("dialog[open]").then(($dialog) => {
      const before = $dialog[0].getBoundingClientRect().height;
      cy.get("dialog[open]").contains("label", "Gender").find("select").select("masculine");
      cy.get('dialog[open] [role="alert"]').should("contain", "The disk is full.");
      cy.get("dialog[open]").should(($now) => expect($now[0].getBoundingClientRect().height).to.equal(before));
    });
  });

  it("styles Root and Verb form like the Add tag input", () => {
    openWord(4210);
    cy.get(tagsButton).click();
    cy.get(addTag).blur();
    const look = ($el: JQuery<HTMLElement>) => {
      const style = getComputedStyle($el[0]);
      return [style.backgroundColor, style.borderTopWidth, style.borderTopColor, style.borderTopLeftRadius, style.paddingLeft, style.minHeight];
    };
    for (const label of ["Root", "Verb form"]) {
      cy.get(`dialog[open] ${by(label)}`).should(($field) => expect(look($field)).to.deep.equal(look(Cypress.$(addTag))));
    }
  });

  it("fits the dialog without a scrollbar at 1280x720, also with a tag", () => {
    cy.viewport(1280, 720);
    openWord(4211);
    setTags(["lesson_1"]);
    reopen();
    cy.get("dialog[open]").should(($dialog) => {
      expect($dialog[0].scrollHeight).to.be.at.most($dialog[0].clientHeight);
    });
  });
});
