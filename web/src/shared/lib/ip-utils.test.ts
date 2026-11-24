import { describe, it, expect } from "vitest";

// IP address utility functions
function isValidIPv4(ip: string): boolean {
  const parts = ip.split(".");
  if (parts.length !== 4) return false;
  return parts.every((part) => {
    const num = Number(part);
    return Number.isInteger(num) && num >= 0 && num <= 255 && part === String(num);
  });
}

function isPrivateIP(ip: string): boolean {
  if (!isValidIPv4(ip)) return false;
  const parts = ip.split(".").map(Number);
  // 10.0.0.0/8
  if (parts[0] === 10) return true;
  // 172.16.0.0/12
  if (parts[0] === 172 && parts[1] >= 16 && parts[1] <= 31) return true;
  // 192.168.0.0/16
  if (parts[0] === 192 && parts[1] === 168) return true;
  return false;
}

function parseCIDR(cidr: string): { ip: string; prefix: number } | null {
  const parts = cidr.split("/");
  if (parts.length !== 2) return null;
  const ip = parts[0];
  const prefix = Number(parts[1]);
  if (!isValidIPv4(ip) || !Number.isInteger(prefix) || prefix < 0 || prefix > 32) return null;
  return { ip, prefix };
}

describe("isValidIPv4", () => {
  it("accepts valid IPv4 addresses", () => {
    expect(isValidIPv4("192.168.1.1")).toBe(true);
    expect(isValidIPv4("10.0.0.1")).toBe(true);
    expect(isValidIPv4("255.255.255.255")).toBe(true);
    expect(isValidIPv4("0.0.0.0")).toBe(true);
  });

  it("rejects invalid IPv4 addresses", () => {
    expect(isValidIPv4("256.1.1.1")).toBe(false);
    expect(isValidIPv4("1.2.3")).toBe(false);
    expect(isValidIPv4("1.2.3.4.5")).toBe(false);
    expect(isValidIPv4("not-an-ip")).toBe(false);
    expect(isValidIPv4("")).toBe(false);
  });
});

describe("isPrivateIP", () => {
  it("identifies 10.x.x.x as private", () => {
    expect(isPrivateIP("10.0.0.1")).toBe(true);
    expect(isPrivateIP("10.255.255.255")).toBe(true);
  });

  it("identifies 172.16-31.x.x as private", () => {
    expect(isPrivateIP("172.16.0.1")).toBe(true);
    expect(isPrivateIP("172.31.255.255")).toBe(true);
    expect(isPrivateIP("172.15.0.1")).toBe(false);
    expect(isPrivateIP("172.32.0.1")).toBe(false);
  });

  it("identifies 192.168.x.x as private", () => {
    expect(isPrivateIP("192.168.1.1")).toBe(true);
    expect(isPrivateIP("192.168.0.0")).toBe(true);
  });

  it("identifies public IPs as not private", () => {
    expect(isPrivateIP("8.8.8.8")).toBe(false);
    expect(isPrivateIP("1.1.1.1")).toBe(false);
  });
});

describe("parseCIDR", () => {
  it("parses valid CIDR notation", () => {
    expect(parseCIDR("10.0.0.0/8")).toEqual({ ip: "10.0.0.0", prefix: 8 });
    expect(parseCIDR("192.168.0.0/16")).toEqual({ ip: "192.168.0.0", prefix: 16 });
    expect(parseCIDR("192.168.1.0/24")).toEqual({ ip: "192.168.1.0", prefix: 24 });
  });

  it("rejects invalid CIDR notation", () => {
    expect(parseCIDR("not-a-cidr")).toBeNull();
    expect(parseCIDR("10.0.0.0")).toBeNull();
    expect(parseCIDR("10.0.0.0/33")).toBeNull();
    expect(parseCIDR("256.0.0.0/8")).toBeNull();
  });
});
