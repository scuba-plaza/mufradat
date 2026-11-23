import type axe from "axe-core";

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Cypress {
    interface Chainable {
      /** Waits until React has attached its handlers to the element, so that an event sent to it is not lost. */
      hydrated(selector: string): Chainable<JQuery<HTMLElement>>;
      /** Opens the card of a word, once the page is ready to take input. */
      openWord(rank: number): Chainable<void>;
      /** Fills the card of a word out over the API, so it has everything an approval needs. */
      finishCard(rank: number): Chainable<void>;
      wordRow(rank: number): Chainable<JQuery<HTMLElement>>;
      pressEscape(): Chainable<void>;
      pressEnter(): Chainable<void>;
      clickBackdrop(): Chainable<void>;
      /** Sets the colour scheme and waits out the colour transitions, so a check that follows sees the settled colours. */
      colorScheme(scheme: "light" | "dark"): Chainable<void>;
      axe(context?: string): Chainable<void>;
      menuItem(name: string): Chainable<void>;
    }
  }
}

Cypress.Commands.add("hydrated", (selector: string) =>
  cy.get(selector).first().should(($el) => expect(Object.keys($el[0]).some((key) => key.startsWith("__reactProps$"))).to.equal(true)),
);

Cypress.Commands.add("openWord", (rank: number) => {
  cy.visit(`/open?rank=${rank}`);
  cy.hydrated('[aria-label="Hint English"]');
});

const mp3 = [0xff, 0xfb, 0x90, 0x00, 1, 2, 3, 4];

Cypress.Commands.add("finishCard", (rank: number) => {
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
  });
});

Cypress.Commands.add("wordRow", (rank: number) =>
  cy.get('[aria-label="Word list"] > li').filter((_, row) => row.querySelector(".num")?.textContent === String(rank)),
);

function pressKey(key: string, code: string, windowsVirtualKeyCode: number, text?: string) {
  for (const type of ["keyDown", "keyUp"]) {
    cy.then(() => Cypress.automation("remote:debugger:protocol", {
      command: "Input.dispatchKeyEvent",
      params: { type, key, code, windowsVirtualKeyCode, ...(type === "keyDown" && text ? { text } : {}) },
    }));
  }
}

Cypress.Commands.add("pressEscape", () => pressKey("Escape", "Escape", 27));
Cypress.Commands.add("pressEnter", () => pressKey("Enter", "Enter", 13, "\r"));

function emulateColorScheme(scheme: "light" | "dark") {
  cy.then(() => Cypress.automation("remote:debugger:protocol", {
    command: "Emulation.setEmulatedMedia",
    params: { features: [{ name: "prefers-color-scheme", value: scheme }] },
  }));
}

Cypress.Commands.add("colorScheme", (scheme: "light" | "dark") => {
  emulateColorScheme(scheme);
  cy.wait(200);
});

afterEach(() => emulateColorScheme("light"));

Cypress.Commands.add("clickBackdrop", () => {
  cy.get("dialog[open]").then(($dialog) => {
    $dialog[0].click();
  });
});

type AxeWindow = Window & { axe: Pick<typeof axe, "run"> };

const AXE_TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];

Cypress.Commands.add("axe", (context?: string) =>
  cy.readFile<string>("node_modules/axe-core/axe.min.js", { log: false }).then((source) =>
    cy.window({ log: false }).then(async (win) => {
      win.eval(source);
      const { violations } = await (win as unknown as AxeWindow).axe.run(context ?? win.document, { runOnly: AXE_TAGS });
      const found = violations.map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`);
      expect(found, "accessibility violations").to.deep.equal([]);
    }),
  ),
);

Cypress.Commands.add("menuItem", (name: string) => {
  cy.hydrated('button[aria-label="Menu"]').click();
  cy.contains("header [popover] .options-item", name).focus();
  cy.pressEnter();
});

export {};
