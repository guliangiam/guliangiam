describe("Organizations Page", () => {
  // ── Unauthenticated Access ──

  it("redirects to login when not authenticated", () => {
    cy.visit("/organizations");
    cy.url().should("include", "/login");
  });

  // ── Authenticated Access ──
  // Note: These tests require a running backend and valid credentials.

  describe("when authenticated", { tags: "@auth" }, () => {
    beforeEach(() => {
      cy.login("admin", "admin");
      cy.url().should("not.include", "/login");
      cy.visit("/organizations");
    });

    it("displays the organizations list page", () => {
      // ResourceListCard renders a card with title
      cy.get(".ant-card").should("be.visible");
    });

    it("displays the new organization button", () => {
      // The toolbar has a Link to /organizations/new
      cy.contains("button", /new/i).should("be.visible");
    });

    it("navigates to create organization page", () => {
      cy.contains("button", /new/i).click();
      cy.url().should("include", "/organizations/new");
    });

    it("displays the search input", () => {
      // ResourceListCard has a search input
      cy.get('input[placeholder*="search"], .ant-input-search input').should("be.visible");
    });

    it("displays the organizations table", () => {
      // ResourceListCard renders an Ant Design Table
      cy.get(".ant-table").should("be.visible");
    });

    it("displays table columns for organization data", () => {
      // The table has columns: name, displayName, defaultApplication, status, websiteUrl, createdAt
      cy.get(".ant-table-thead th").should("have.length.greaterThan", 2);
    });

    it("can search organizations", () => {
      // Type in the search input
      cy.get('input[placeholder*="search"], .ant-input-search input')
        .first()
        .type("test-org{enter}");
      // The table should refresh with filtered results
      cy.get(".ant-table").should("be.visible");
    });

    it("can navigate to organization detail", () => {
      // Click the edit link on the first row
      cy.get(".ant-table-tbody tr:first-child a").first().click();
      cy.url().should("match", /\/organizations\//);
    });

    it("can filter by status", () => {
      // Click the status filter dropdown
      cy.get(".ant-select").first().click();
      // Select "Active" option
      cy.contains(".ant-select-item-option", /active/i).click();
      // Table should refresh
      cy.get(".ant-table").should("be.visible");
    });
  });
});