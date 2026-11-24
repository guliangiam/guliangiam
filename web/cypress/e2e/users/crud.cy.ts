describe("Users CRUD", () => {
  // ── Unauthenticated Access ──

  it("redirects to login when visiting users page unauthenticated", () => {
    cy.visit("/users");
    cy.url().should("include", "/login");
  });

  // ── Authenticated CRUD Operations ──

  describe("when authenticated", { tags: "@auth" }, () => {
    beforeEach(() => {
      cy.login("admin", "admin");
      cy.url().should("not.include", "/login");
      cy.visit("/users");
    });

    // ── Read (List) ──

    it("displays the users list page", () => {
      cy.get(".ant-card").should("be.visible");
    });

    it("displays the users table", () => {
      cy.get(".ant-table").should("be.visible");
    });

    it("displays table columns for user data", () => {
      // UsersPage has columns: avatar, username, organization, displayName, email, phone, etc.
      cy.get(".ant-table-thead th").should("have.length.greaterThan", 5);
    });

    it("displays the new user button", () => {
      // The toolbar has a Link to /users/new
      cy.contains("button", /new/i).should("be.visible");
    });

    it("displays the import button", () => {
      // The toolbar has an import button with UploadOutlined icon
      cy.contains("button", /import/i).should("be.visible");
    });

    it("displays the export button", () => {
      // The toolbar has an export button with DownloadOutlined icon
      cy.contains("button", /export/i).should("be.visible");
    });

    it("displays the search input", () => {
      // ResourceListCard has a search input
      cy.get('input[placeholder*="search"], .ant-input-search input').should("be.visible");
    });

    it("can search users by name", () => {
      cy.get('input[placeholder*="search"], .ant-input-search input')
        .first()
        .type("admin{enter}");
      cy.get(".ant-table").should("be.visible");
    });

    it("can filter users by status", () => {
      // Click the status filter dropdown
      cy.get(".ant-select").first().click();
      cy.contains(".ant-select-item-option", /active/i).click();
      cy.get(".ant-table").should("be.visible");
    });

    // ── Create ──

    it("navigates to create user page", () => {
      cy.contains("button", /new/i).click();
      cy.url().should("include", "/users/new");
    });

    // ── Read (Detail) ──

    it("can navigate to user detail by clicking username link", () => {
      // The username column renders a Link to /users/:orgName/:userName
      cy.get(".ant-table-tbody tr:first-child a").first().click();
      cy.url().should("match", /\/users\//);
    });

    // ── Delete ──

    it("displays delete button for each user row", () => {
      // Each row has a delete button with DeleteOutlined icon
      cy.get(".ant-table-tbody tr:first-child .anticon-delete").should("exist");
    });

    it("shows confirmation dialog before deleting a user", () => {
      // Click the delete icon button in the first row
      cy.get(".ant-table-tbody tr:first-child .anticon-delete").click();
      // Ant Design Popconfirm should appear
      cy.get(".ant-popconfirm").should("be.visible");
      // Cancel the deletion
      cy.contains(".ant-popconfirm button", /cancel/i).click();
    });

    // ── Import ──

    it("opens import modal when clicking import button", () => {
      cy.contains("button", /import/i).click();
      // BatchImportModal should appear
      cy.get(".ant-modal").should("be.visible");
      // Close the modal
      cy.get(".ant-modal-close").click();
    });

    // ── Bulk Actions ──

    it("can select multiple users for bulk operations", () => {
      // Select the first row checkbox
      cy.get(".ant-table-tbody .ant-checkbox-input").first().check({ force: true });
      // Bulk action buttons should appear
      cy.contains("button", /delete/i).should("be.visible");
    });
  });
});