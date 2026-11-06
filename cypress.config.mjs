import fs from "node:fs";
import path from "node:path";
import { defineConfig } from "cypress";

export default defineConfig({
  video: false,
  viewportWidth: 1280,
  viewportHeight: 800,
  e2e: {
    specPattern: "cypress/e2e/**/*.cy.ts",
    supportFile: "cypress/support/e2e.ts",
    setupNodeEvents(on, config) {
      on("task", {
        removeAudio(key) {
          fs.rmSync(path.join(config.env.WORKSPACE, "audio", key.slice(0, 2), `${key}.mp3`), { force: true });
          return null;
        },
      });
      return config;
    },
  },
});
