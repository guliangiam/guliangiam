/// <reference types="cypress" />
import "@testing-library/cypress/add-commands";

// ── Custom Commands ──

/**
 * Login via UI with the given credentials.
 * Assumes the login form uses Ant Design Form with username/password fields.
 */
Cypress.Commands.add("login", (username: string, password: string) => {
  cy.visit("/login");
  // Wait for the login form to render (Ant Design Form)
  cy.get("form").should("be.visible");
  // Fill in username - Ant Design Input uses input element inside .ant-input
  cy.get('input[id="username"], input[name="username"]').clear().type(username);
  // Fill in password - Ant Design Password uses input inside .ant-input-password
  cy.get('input[id="password"], input[type="password"]').clear().type(password);
  // Submit the form
  cy.get('button[type="submit"]').click();
});

/**
 * Login via API (programmatic login) to skip UI for faster tests.
 * Sets session state directly so the app thinks the user is authenticated.
 */
Cypress.Commands.add("loginByApi", (username: string, password: string) => {
  cy.request({
    method: "POST",
    url: "http://localhost:8000/api/login",
    body: { username, password },
    failOnStatusCode: false,
  }).then((response) => {
    if (response.status === 200) {
      // Store the session token if returned
      const token = response.body?.token || response.body?.data?.token;
      if (token) {
        window.localStorage.setItem("token", token);
      }
    }
  });
});

/**
 * Logout via UI - click the logout button in the header.
 */
Cypress.Commands.add("logout", () => {
  // The logout button is in the header area
  cy.get("button").contains(/logout|sign out|退出/i).click();
  // Should redirect to login page
  cy.url().should("include", "/login");
});

// ── Type Augmentation ──
declare global {
  namespace Cypress {
    interface Chainable {
      login(username: string, password: string): Chainable<void>;
      loginByApi(username: string, password: string): Chainable<void>;
      logout(): Chainable<void>;
    }
  }
}