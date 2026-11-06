// Runs the Cypress suite against a production build of the app, started here with
// an empty workspace and no API credentials, so a test can never touch real data
// or spend money. Usage: node scripts/e2e.mjs run|open [Cypress options, e.g. --spec cypress/e2e/list.cy.ts]
// The browser is the Chrome for Testing version pinned in cypress/browser.json, downloaded once into
// ~/.cache/mufradat/browsers, so every machine and CI run the same build.
import { Browser, install } from "@puppeteer/browsers";
import { spawn, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const mode = process.argv[2] === "open" ? "open" : "run";
const port = Number(process.env.E2E_PORT ?? 3100);
const baseUrl = `http://127.0.0.1:${port}`;
const extra = process.argv.slice(3);
const root = path.resolve(import.meta.dirname, "..");
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), "mufradat-e2e-"));

const alive = () => fetch(baseUrl).then(() => true, () => false);
const finish = (code) => {
  server?.kill();
  fs.rmSync(scratch, { recursive: true, force: true });
  process.exit(code);
};
let server;
process.on("SIGINT", () => finish(130));
process.on("SIGTERM", () => finish(143));

if (await alive()) {
  console.error(`Something already answers on ${baseUrl}; stop it or set E2E_PORT.`);
  finish(1);
}

const { chrome } = JSON.parse(fs.readFileSync(path.join(root, "cypress", "browser.json"), "utf8"));
const { executablePath: browser } = await install({
  browser: Browser.CHROME,
  buildId: chrome,
  cacheDir: path.join(os.homedir(), ".cache", "mufradat", "browsers"),
});

if (!process.env.E2E_SKIP_BUILD) {
  const build = spawnSync("pnpm", ["build"], { cwd: root, stdio: "inherit" });
  if (build.status !== 0) finish(build.status ?? 1);
}

const env = { ...process.env, PORT: String(port), MUFRADAT_WORKSPACE: path.join(scratch, "workspace"), XDG_CONFIG_HOME: path.join(scratch, "config") };
delete env.MISTRAL_API_KEY;
delete env.GOOGLE_APPLICATION_CREDENTIALS;
server = spawn(path.join(root, "bin", "mufradat"), { cwd: root, env, stdio: "ignore" });
let exited = false;
server.on("exit", () => (exited = true));

for (let i = 0; !(await alive()); i++) {
  if (exited || i > 120) {
    console.error("The app did not start.");
    finish(1);
  }
  await new Promise((r) => setTimeout(r, 500));
}

const cypress = spawn("pnpm", ["exec", "cypress", mode, "--e2e", "--browser", browser, "--config", `baseUrl=${baseUrl}`, ...extra], {
  cwd: root,
  stdio: "inherit",
  env: { ...process.env, CYPRESS_WORKSPACE: env.MUFRADAT_WORKSPACE },
});
cypress.on("exit", (code) => finish(code ?? 1));
