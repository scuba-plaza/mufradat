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
      const apkgs = (dir) => (fs.existsSync(dir) ? fs.readdirSync(dir).filter((name) => name.endsWith(".apkg")) : []);
      on("task", {
        async downloads(atLeast = 0) {
          for (let i = 0; i < 40; i++) {
            const names = apkgs(config.downloadsFolder);
            if (names.length >= atLeast) return names;
            await new Promise((resolve) => setTimeout(resolve, 250));
          }
          return apkgs(config.downloadsFolder);
        },
        workspaceExports() {
          const dir = path.join(config.env.WORKSPACE, "exports");
          return fs.existsSync(dir) ? fs.readdirSync(dir) : [];
        },
        removeAudio(key) {
          fs.rmSync(path.join(config.env.WORKSPACE, "audio", key.slice(0, 2), `${key}.mp3`), { force: true });
          return null;
        },
      });
      return config;
    },
  },
});
