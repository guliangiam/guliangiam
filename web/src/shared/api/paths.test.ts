import { describe, it, expect } from "vitest";
import { RECORDS, RECORDS_SEARCH, RECORDS_AUDIT_LOGS, RECORDS_LOGIN_LOGS } from "./paths";

// paths.ts is the single source of API paths (check-paths-consistency). These
// assertions pin the records family to the routes the server registers in
// internal/transport/http/api/v1/records/routes.go, so a rename on either side
// has to be deliberate.
describe("records path constants", () => {
  it("declares the records collection and search endpoints", () => {
    expect(RECORDS).toBe("/api/v1/records");
    expect(RECORDS_SEARCH).toBe("/api/v1/records/search");
  });

  it("declares the audit-center category log endpoints", () => {
    expect(RECORDS_AUDIT_LOGS).toBe("/api/v1/records/audit-logs");
    expect(RECORDS_LOGIN_LOGS).toBe("/api/v1/records/login-logs");
  });
});
