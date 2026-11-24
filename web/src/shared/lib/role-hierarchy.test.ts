import { describe, it, expect } from "vitest";

// Role hierarchy: admin > editor > viewer
type Role = "admin" | "editor" | "viewer";

const roleHierarchy: Record<Role, number> = {
  viewer: 0,
  editor: 1,
  admin: 2,
};

function hasPermission(userRole: Role, requiredRole: Role): boolean {
  return roleHierarchy[userRole] >= roleHierarchy[requiredRole];
}

function canCreate(role: Role): boolean {
  return hasPermission(role, "editor");
}

function canDelete(role: Role): boolean {
  return hasPermission(role, "admin");
}

function canView(role: Role): boolean {
  return hasPermission(role, "viewer");
}

describe("role hierarchy", () => {
  it("admin has all permissions", () => {
    expect(canView("admin")).toBe(true);
    expect(canCreate("admin")).toBe(true);
    expect(canDelete("admin")).toBe(true);
  });

  it("editor can view and create but not delete", () => {
    expect(canView("editor")).toBe(true);
    expect(canCreate("editor")).toBe(true);
    expect(canDelete("editor")).toBe(false);
  });

  it("viewer can only view", () => {
    expect(canView("viewer")).toBe(true);
    expect(canCreate("viewer")).toBe(false);
    expect(canDelete("viewer")).toBe(false);
  });
});

describe("hasPermission", () => {
  it("admin passes all checks", () => {
    expect(hasPermission("admin", "viewer")).toBe(true);
    expect(hasPermission("admin", "editor")).toBe(true);
    expect(hasPermission("admin", "admin")).toBe(true);
  });

  it("editor passes viewer and editor checks", () => {
    expect(hasPermission("editor", "viewer")).toBe(true);
    expect(hasPermission("editor", "editor")).toBe(true);
    expect(hasPermission("editor", "admin")).toBe(false);
  });

  it("viewer only passes viewer check", () => {
    expect(hasPermission("viewer", "viewer")).toBe(true);
    expect(hasPermission("viewer", "editor")).toBe(false);
    expect(hasPermission("viewer", "admin")).toBe(false);
  });
});
