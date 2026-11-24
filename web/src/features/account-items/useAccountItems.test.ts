import { describe, it, expect } from "vitest";
import {
  useAccountItems,
  isFieldVisible,
  isFieldDisabled,
  getFieldRegex,
  getUniqueTabs,
  getAccountItemsByTab,
  mapTabNameToKey,
  getFieldTabKey,
} from "./useAccountItems";
import type { AccountItem } from "@/shared/api/iam/organization";
import { renderHook } from "@testing-library/react";

function makeItem(overrides: Partial<AccountItem>): AccountItem {
  return {
    name: "Username",
    visible: true,
    viewRule: "Self",
    modifyRule: "Admin",
    regex: "",
    tab: "basic",
    ...overrides,
  };
}

describe("useAccountItems", () => {
  it("returns empty map for undefined accountItems", () => {
    const { result } = renderHook(() =>
      useAccountItems({ accountItems: undefined, viewerRole: "admin" }),
    );
    expect(result.current).toEqual({});
  });

  it("returns empty map for empty array", () => {
    const { result } = renderHook(() => useAccountItems({ accountItems: [], viewerRole: "admin" }));
    expect(result.current).toEqual({});
  });

  it("maps visible and modifyRule for admin", () => {
    const items: AccountItem[] = [
      makeItem({ name: "Email", visible: true, viewRule: "Self", modifyRule: "Admin" }),
      makeItem({ name: "Phone", visible: true, viewRule: "Admin", modifyRule: "Immutable" }),
    ];
    const { result } = renderHook(() =>
      useAccountItems({ accountItems: items, viewerRole: "admin" }),
    );
    expect(result.current["Email"]).toEqual({ visible: true, disabled: false });
    expect(result.current["Phone"]).toEqual({ visible: true, disabled: true });
  });

  it("applies viewRule=Admin for self viewer", () => {
    const items: AccountItem[] = [
      makeItem({ name: "Score", visible: true, viewRule: "Admin", modifyRule: "Admin" }),
    ];
    const { result } = renderHook(() =>
      useAccountItems({ accountItems: items, viewerRole: "self" }),
    );
    expect(result.current["Score"].visible).toBe(false);
  });

  it("applies modifyRule=Immutable", () => {
    const items: AccountItem[] = [
      makeItem({ name: "ID card", visible: true, viewRule: "Self", modifyRule: "Immutable" }),
    ];
    const { result } = renderHook(() =>
      useAccountItems({ accountItems: items, viewerRole: "admin" }),
    );
    expect(result.current["ID card"].disabled).toBe(true);
  });

  it("preserves regex", () => {
    const items: AccountItem[] = [
      makeItem({
        name: "Phone",
        visible: true,
        viewRule: "Self",
        modifyRule: "Admin",
        regex: "^\\+?[0-9]+$",
      }),
    ];
    const { result } = renderHook(() =>
      useAccountItems({ accountItems: items, viewerRole: "self" }),
    );
    expect(result.current["Phone"].regex).toBe("^\\+?[0-9]+$");
  });
});

describe("isFieldVisible", () => {
  it("returns true for empty map", () => {
    expect(isFieldVisible({}, "any")).toBe(true);
  });

  it("returns true for undefined field", () => {
    expect(isFieldVisible({ Email: { visible: true, disabled: false } }, "Phone")).toBe(true);
  });

  it("returns field visibility", () => {
    const map = { Email: { visible: false, disabled: false } };
    expect(isFieldVisible(map, "Email")).toBe(false);
  });
});

describe("isFieldDisabled", () => {
  it("returns false for empty map", () => {
    expect(isFieldDisabled({}, "any")).toBe(false);
  });

  it("returns field disabled state", () => {
    const map = { Phone: { visible: true, disabled: true } };
    expect(isFieldDisabled(map, "Phone")).toBe(true);
  });
});

describe("getFieldRegex", () => {
  it("returns undefined for empty map", () => {
    expect(getFieldRegex({}, "any")).toBeUndefined();
  });

  it("returns regex when set", () => {
    const map = { Phone: { visible: true, disabled: false, regex: "^\\d+$" } };
    expect(getFieldRegex(map, "Phone")).toBe("^\\d+$");
  });
});

describe("getUniqueTabs", () => {
  it("returns empty for undefined", () => {
    expect(getUniqueTabs(undefined)).toEqual([]);
  });

  it("extracts unique tabs preserving order", () => {
    const items: AccountItem[] = [
      makeItem({ name: "A", tab: "basic" }),
      makeItem({ name: "B", tab: "profile" }),
      makeItem({ name: "C", tab: "basic" }),
    ];
    expect(getUniqueTabs(items)).toEqual(["basic", "profile"]);
  });
});

describe("getAccountItemsByTab", () => {
  it("filters items by tab", () => {
    const items: AccountItem[] = [
      makeItem({ name: "A", tab: "basic" }),
      makeItem({ name: "B", tab: "profile" }),
      makeItem({ name: "C", tab: "basic" }),
    ];
    expect(getAccountItemsByTab(items, "basic")).toHaveLength(2);
    expect(getAccountItemsByTab(items, "profile")).toHaveLength(1);
  });
});

describe("mapTabNameToKey", () => {
  it("maps basic/info to basic", () => {
    expect(mapTabNameToKey("Basic Information")).toBe("basic");
    expect(mapTabNameToKey("User Info")).toBe("basic");
  });

  it("maps profile/personal/contact into the merged basic section", () => {
    expect(mapTabNameToKey("Profile Details")).toBe("basic");
    expect(mapTabNameToKey("Personal")).toBe("basic");
    expect(mapTabNameToKey("Contact")).toBe("basic");
  });

  it("maps real/id/identity to realname", () => {
    expect(mapTabNameToKey("Real Name")).toBe("realname");
    expect(mapTabNameToKey("ID Verification")).toBe("realname");
  });

  it("maps credential/password/security to credentials", () => {
    expect(mapTabNameToKey("Credentials")).toBe("credentials");
    expect(mapTabNameToKey("Password")).toBe("credentials");
  });

  it("maps session/consent into sessions and finance/managed into assets", () => {
    expect(mapTabNameToKey("Sessions")).toBe("sessions");
    expect(mapTabNameToKey("Consents")).toBe("sessions");
    expect(mapTabNameToKey("Finance")).toBe("assets");
    expect(mapTabNameToKey("Managed Accounts")).toBe("assets");
  });

  it("defaults to basic for unknown", () => {
    expect(mapTabNameToKey("unknown")).toBe("basic");
  });
});

describe("getFieldTabKey", () => {
  it("maps known fields", () => {
    expect(getFieldTabKey("Username")).toBe("basic");
    expect(getFieldTabKey("Email")).toBe("basic");
    expect(getFieldTabKey("Balance")).toBe("assets");
    expect(getFieldTabKey("Password")).toBe("credentials");
    expect(getFieldTabKey("Real name")).toBe("realname");
  });

  it("defaults to basic for unknown field", () => {
    expect(getFieldTabKey("CustomField")).toBe("basic");
  });
});
