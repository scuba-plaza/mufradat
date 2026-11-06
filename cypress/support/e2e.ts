import type { AxeResults } from "axe-core";

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Cypress {
    interface Chainable {
      /** Waits until React has attached its handlers to the element, so that an event sent to it is not lost. */
      hydrated(selector: string): Chainable<JQuery<HTMLElement>>;
      wordRow(rank: number): Chainable<JQuery<HTMLElement>>;
      pressEscape(): Chainable<void>;
      axe(context?: string): Chainable<void>;
      menuItem(name: string): Chainable<void>;
    }
  }
}

Cypress.Commands.add("hydrated", (selector: string) =>
  cy.get(selector).first().should(($el) => expect(Object.keys($el[0]).some((key) => key.startsWith("__reactProps$"))).to.equal(true)),
);

Cypress.Commands.add("wordRow", (rank: number) =>
  cy.get('[aria-label="Word list"] > li').filter((_, row) => row.querySelector(".num")?.textContent === String(rank)),
);

Cypress.Commands.add("pressEscape", () => {
  for (const type of ["keyDown", "keyUp"]) {
    cy.then(() => Cypress.automation("remote:debugger:protocol", {
      command: "Input.dispatchKeyEvent",
      params: { type, key: "Escape", code: "Escape", windowsVirtualKeyCode: 27 },
    }));
  }
});

type AxeWindow = Window & { axe: { run: (context: string | Document, options: { runOnly: string[] }) => Promise<AxeResults> } };

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
  cy.contains("header [popover] .options-item", name).click();
});

export {};
