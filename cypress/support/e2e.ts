declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Cypress {
    interface Chainable {
      /** Waits until React has attached its handlers to the element, so that an event sent to it is not lost. */
      hydrated(selector: string): Chainable<JQuery<HTMLElement>>;
      wordRow(rank: number): Chainable<JQuery<HTMLElement>>;
    }
  }
}

Cypress.Commands.add("hydrated", (selector: string) =>
  cy.get(selector).first().should(($el) => expect(Object.keys($el[0]).some((key) => key.startsWith("__reactProps$"))).to.equal(true)),
);

Cypress.Commands.add("wordRow", (rank: number) =>
  cy.get('[aria-label="Word list"] > li').filter((_, row) => row.querySelector(".num")?.textContent === String(rank)),
);

export {};
