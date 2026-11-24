import { describe, it, expect, beforeEach } from "vitest";
import { useSessionStore } from "./session-store";

// Reset store between tests
beforeEach(() => {
  useSessionStore.setState({
    account: null,
    organization: null,
    passwordPolicy: null,
    needUpdatePassword: null,
    impersonatedUser: null,
    impersonationToken: null,
  });
});

describe("useSessionStore", () => {
  it("initializes with null account and organization", () => {
    const state = useSessionStore.getState();
    expect(state.account).toBeNull();
    expect(state.organization).toBeNull();
    expect(state.impersonatedUser).toBeNull();
  });

  it("sets session with account and organization", () => {
    const account = {
      owner: "org-1",
      name: "admin",
      displayName: "Admin User",
      avatar: null,
      isAdmin: true,
    };
    const organization = {
      id: "org-1",
      name: "default",
      displayName: "Default Org",
      logoUrl: null,
      faviconUrl: null,
      navItems: [],
      userNavItems: [],
    };

    useSessionStore.getState().setSession({ account, organization });

    const state = useSessionStore.getState();
    expect(state.account).toEqual(account);
    expect(state.organization).toEqual(organization);
  });

  it("clears session by setting null values", () => {
    // First set some values
    useSessionStore.getState().setSession({
      account: {
        owner: "org-1",
        name: "admin",
        displayName: "Admin",
        avatar: null,
        isAdmin: true,
      },
      organization: {
        id: "org-1",
        name: "default",
        displayName: "Default",
        navItems: [],
        userNavItems: [],
      },
    });

    // Then clear
    useSessionStore.getState().setSession({ account: null, organization: null });

    const state = useSessionStore.getState();
    expect(state.account).toBeNull();
    expect(state.organization).toBeNull();
  });

  it("sets impersonated user", () => {
    const impersonatedUser = {
      id: "user-1",
      username: "impuser",
      displayName: "Impersonated User",
    };

    useSessionStore.getState().setImpersonatedUser(impersonatedUser);

    const state = useSessionStore.getState();
    expect(state.impersonatedUser).toEqual(impersonatedUser);
  });

  it("clears impersonated user by setting null", () => {
    useSessionStore.getState().setImpersonatedUser({
      id: "user-1",
      username: "impuser",
      displayName: "Impersonated User",
    });

    useSessionStore.getState().setImpersonatedUser(null);

    const state = useSessionStore.getState();
    expect(state.impersonatedUser).toBeNull();
  });

  it("setSession does not affect impersonatedUser", () => {
    useSessionStore.getState().setImpersonatedUser({
      id: "user-1",
      username: "impuser",
      displayName: "Impersonated User",
    });

    useSessionStore.getState().setSession({
      account: {
        owner: "org-1",
        name: "admin",
        displayName: "Admin",
        avatar: null,
        isAdmin: true,
      },
      organization: {
        id: "org-1",
        name: "default",
        displayName: "Default",
        navItems: [],
        userNavItems: [],
      },
    });

    const state = useSessionStore.getState();
    expect(state.impersonatedUser).not.toBeNull();
    expect(state.impersonatedUser?.username).toBe("impuser");
  });

  it("setImpersonatedUser does not affect account or organization", () => {
    useSessionStore.getState().setSession({
      account: {
        owner: "org-1",
        name: "admin",
        displayName: "Admin",
        avatar: null,
        isAdmin: true,
      },
      organization: {
        id: "org-1",
        name: "default",
        displayName: "Default",
        navItems: [],
        userNavItems: [],
      },
    });

    useSessionStore.getState().setImpersonatedUser({
      id: "user-1",
      username: "impuser",
      displayName: "Impersonated User",
    });

    const state = useSessionStore.getState();
    expect(state.account).not.toBeNull();
    expect(state.organization).not.toBeNull();
  });

  it("keeps needUpdatePassword when setSession omits it", () => {
    useSessionStore.getState().setSession({
      account: null,
      organization: null,
      needUpdatePassword: true,
    });

    useSessionStore.getState().setSession({
      account: {
        owner: "org-1",
        name: "admin",
        displayName: "Admin",
        avatar: null,
        isAdmin: true,
      },
      organization: null,
    });

    expect(useSessionStore.getState().needUpdatePassword).toBe(true);
  });

  it("clears needUpdatePassword when explicitly set to null", () => {
    useSessionStore.getState().setSession({
      account: null,
      organization: null,
      needUpdatePassword: true,
    });

    useSessionStore.getState().setSession({
      account: null,
      organization: null,
      needUpdatePassword: null,
    });

    expect(useSessionStore.getState().needUpdatePassword).toBeNull();
  });
});

describe("impersonation token lifecycle", () => {
  it("stores the imp token in memory alongside the impersonated user", () => {
    const store = useSessionStore.getState();
    store.setImpersonatedUser({ id: "user-1", username: "bob", displayName: "Bob" });
    store.setImpersonationToken("imp_test_token");
    expect(useSessionStore.getState().impersonationToken).toBe("imp_test_token");
  });

  it("clears the token when the impersonated user is cleared", () => {
    const store = useSessionStore.getState();
    store.setImpersonatedUser({ id: "user-1", username: "bob", displayName: "Bob" });
    store.setImpersonationToken("imp_test_token");
    store.setImpersonatedUser(null);
    const state = useSessionStore.getState();
    expect(state.impersonatedUser).toBeNull();
    expect(state.impersonationToken).toBeNull();
  });

  it("clears the target-derived password gate flag when impersonation ends", () => {
    const store = useSessionStore.getState();
    store.setImpersonatedUser({ id: "user-1", username: "bob", displayName: "Bob" });
    useSessionStore.setState({ needUpdatePassword: true });
    store.setImpersonatedUser(null);
    expect(useSessionStore.getState().needUpdatePassword).toBeNull();
  });

  it("keeps the token when switching to another impersonated user", () => {
    const store = useSessionStore.getState();
    store.setImpersonationToken("imp_test_token");
    store.setImpersonatedUser({ id: "user-2", username: "carol", displayName: "Carol" });
    expect(useSessionStore.getState().impersonationToken).toBe("imp_test_token");
  });
});
