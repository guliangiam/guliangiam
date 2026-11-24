describe("Organization Switch", () => {
  // ── Unauthenticated Access ──

  it("redirects to login when not authenticated", () => {
    cy.visit("/");
    cy.url().should("include", "/login");
  });

  // ── Authenticated Organization Switching ──

  describe("when authenticated", { tags: "@auth" }, () => {
    beforeEach(() => {
      cy.login("admin", "admin");
      cy.url().should("not.include", "/login");
      cy.visit("/");
    });

    it("displays the current organization in the header", () => {
      // The header has an organization Select component
      cy.get(".gl-header__meta .ant-select").should("be.visible");
    });

    it("displays the organization name in the sidebar brand", () => {
      // sidebarBrand renders .gl-sidebar__org with org displayName
      cy.get(".gl-sidebar__org").should("be.visible");
    });

    it("can open organization selector dropdown", () => {
      // Click the organization Select in the header
      cy.get(".gl-header__meta .ant-select").click();
      // Dropdown should appear
      cy.get(".ant-select-dropdown").should("be.visible");
    });

    it("reflects the current organization in the header avatar", () => {
      // The header brand has an Avatar with org name initial
      cy.get(".gl-header__brand-icon .ant-avatar").should("be.visible");
    });
  });
});