describe("Sidebar Navigation", () => {
  // ── Unauthenticated Access ──

  it("redirects to login when visiting root unauthenticated", () => {
    cy.visit("/");
    cy.url().should("include", "/login");
  });

  it("redirects to login when visiting any protected route unauthenticated", () => {
    const protectedRoutes = ["/organizations", "/users", "/applications", "/roles"];
    protectedRoutes.forEach((route) => {
      cy.visit(route);
      cy.url().should("include", "/login");
    });
  });

  // ── Authenticated Navigation ──
  // Note: These tests require a running backend and valid credentials.
  // They are tagged with @auth for selective execution.

  describe("when authenticated", { tags: "@auth" }, () => {
    beforeEach(() => {
      // Attempt programmatic login - will only work with a running backend
      cy.login("admin", "admin");
      // Wait for redirect after successful login
      cy.url().should("not.include", "/login");
    });

    it("displays the sidebar with menu groups", () => {
      // The sidebar is rendered with Ant Design Menu inside .gl-sidebar
      cy.get(".gl-sidebar").should("be.visible");
    });

    it("displays the admin console title in sidebar brand", () => {
      // sidebarBrand renders .gl-sidebar__title with "Admin Console"
      cy.get(".gl-sidebar__title").should("be.visible");
    });

    it("displays the organization name in sidebar", () => {
      // sidebarBrand renders .gl-sidebar__org with org displayName
      cy.get(".gl-sidebar__org").should("be.visible");
    });

    it("navigates to organizations page via menu", () => {
      // Click the Organizations menu item in the identity group
      cy.contains(".ant-menu-item", /organizations/i).click();
      cy.url().should("include", "/organizations");
    });

    it("navigates to users page via menu", () => {
      cy.contains(".ant-menu-item", /users/i).click();
      cy.url().should("include", "/users");
    });

    it("can toggle sidebar collapse", () => {
      // The toggle button is .gl-sidebar__toggle-btn
      cy.get(".gl-sidebar__toggle-btn").click();
      // Sidebar should be collapsed (narrow width)
      cy.get(".gl-sidebar--inline").should("have.class", "ant-layout-sider-collapsed");
      // Click again to expand
      cy.get(".gl-sidebar__toggle-btn").click();
      cy.get(".gl-sidebar--inline").should("not.have.class", "ant-layout-sider-collapsed");
    });

    it("can switch layout mode between sidebar and top nav", () => {
      // The SwapOutlined button toggles layout mode
      cy.get(".gl-header__meta button").first().click();
      // Should switch to top nav layout
      cy.get(".gl-app-shell--top").should("exist");
      // Switch back
      cy.get(".gl-header__meta button").first().click();
      cy.get(".gl-app-shell--top").should("not.exist");
    });

    it("can logout from header", () => {
      cy.logout();
      cy.url().should("include", "/login");
    });
  });
});