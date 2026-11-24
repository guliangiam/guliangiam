import { defineConfig } from "cypress";

export default defineConfig({
  e2e: {
    baseUrl: "http://localhost:7001",
    viewportWidth: 1280,
    viewportHeight: 720,
    video: false,
    screenshotOnRunFailure: true,
    defaultCommandTimeout: 10000,
    supportFile: "cypress/support/e2e.ts",
    specPattern: "cypress/e2e/**/*.cy.{ts,tsx}",
    // Avoid failing on uncaught exceptions from the app (e.g. React error boundaries)
    setupNodeEvents(on, _config) {
      on("task", {
        log(message: unknown) {
          console.log(message);
          return null;
        },
      });
    },
  },
});
