import { defineConfig } from "cypress";

export default defineConfig({
  video: false,
  viewportWidth: 1280,
  viewportHeight: 800,
  e2e: {
    specPattern: "cypress/e2e/**/*.cy.ts",
    supportFile: "cypress/support/e2e.ts",
  },
});
