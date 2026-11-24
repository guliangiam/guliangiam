import { describe, it, expect } from "vitest";

// QR Code session state machine
type QRCodeStatus = "pending" | "scanned" | "confirmed" | "exchanged" | "expired";

const qrValidTransitions: Record<QRCodeStatus, QRCodeStatus[]> = {
  pending: ["scanned", "expired"],
  scanned: ["confirmed", "expired"],
  confirmed: ["exchanged", "expired"],
  exchanged: [], // terminal
  expired: [], // terminal
};

function canQRTransition(from: QRCodeStatus, to: QRCodeStatus): boolean {
  return qrValidTransitions[from].includes(to);
}

function isQRTerminal(status: QRCodeStatus): boolean {
  return qrValidTransitions[status].length === 0;
}

describe("QR Code state machine", () => {
  it("pending → scanned", () => {
    expect(canQRTransition("pending", "scanned")).toBe(true);
  });

  it("pending → expired", () => {
    expect(canQRTransition("pending", "expired")).toBe(true);
  });

  it("scanned → confirmed", () => {
    expect(canQRTransition("scanned", "confirmed")).toBe(true);
  });

  it("confirmed → exchanged", () => {
    expect(canQRTransition("confirmed", "exchanged")).toBe(true);
  });

  it("cannot skip states (pending → confirmed)", () => {
    expect(canQRTransition("pending", "confirmed")).toBe(false);
  });

  it("cannot go backward (scanned → pending)", () => {
    expect(canQRTransition("scanned", "pending")).toBe(false);
  });

  it("exchanged is terminal", () => {
    expect(isQRTerminal("exchanged")).toBe(true);
  });

  it("expired is terminal", () => {
    expect(isQRTerminal("expired")).toBe(true);
  });

  it("pending is not terminal", () => {
    expect(isQRTerminal("pending")).toBe(false);
  });
});
