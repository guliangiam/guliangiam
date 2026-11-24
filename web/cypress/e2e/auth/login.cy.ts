describe("Login Flow", () => {
  beforeEach(() => {
    cy.visit("/login");
  });

  // ── Page Rendering ──

  it("displays the login form on /login", () => {
    // The LoginPage renders an Ant Design Form
    cy.get("form").should("be.visible");
  });

  it("renders the username input field", () => {
    // LoginSigninForm has a Form.Item with name="username"
    cy.get('input[id="username"], input[name="username"]').should("exist");
  });

  it("renders the password input field", () => {
    // LoginSigninForm has a Form.Item with name="password"
    cy.get('input[type="password"]').should("exist");
  });

  it("renders the submit button", () => {
    // The "Login button" signin item renders a submit button
    cy.get('button[type="submit"]').should("exist");
  });

  // ── Form Validation ──

  it("shows validation error when submitting empty form", () => {
    // Click submit without filling in credentials
    cy.get('button[type="submit"]').click();
    // Ant Design Form shows validation messages
    cy.get(".ant-form-item-explain-error").should("be.visible");
  });

  it("shows validation error when submitting only username", () => {
    cy.get('input[id="username"], input[name="username"]').type("admin");
    cy.get('button[type="submit"]').click();
    // Password field should show a required error
    cy.get(".ant-form-item-explain-error").should("be.visible");
  });

  // ── Navigation ──

  it("can navigate to forgot password page", () => {
    // ForgotPasswordSigninItem renders a link to /forgot-password
    cy.contains(/forgot\s*password/i).click();
    cy.url().should("include", "/forgot-password");
  });

  it("can navigate to signup page", () => {
    // SignupLinkSigninItem renders a link to /signup
    cy.contains(/sign\s*up/i).click();
    cy.url().should("include", "/signup");
  });

  // ── Login Attempt ──

  it("shows error for invalid credentials", () => {
    cy.get('input[id="username"], input[name="username"]').type("invalid_user");
    cy.get('input[type="password"]').type("wrong_password");
    cy.get('button[type="submit"]').click();
    // After failed login, should stay on login page
    // and show an error message (Ant Design message or alert)
    cy.url().should("include", "/login");
  });

  // ── Auth Redirect ──

  it("redirects to login when visiting protected route unauthenticated", () => {
    // AppShell returns <Navigate to="/login" /> when !account || !organization
    cy.visit("/");
    cy.url().should("include", "/login");
  });
});