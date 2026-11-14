import { defineConfig } from "vitest/config";

export default defineConfig({
  test: { environment: "node", include: ["bench/**/*.bench.ts"], testTimeout: 3_600_000 },
});
