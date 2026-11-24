import { z } from "zod";
import { SYSTEM_HEALTH, SYSTEM_INFO, SYSTEM_METRICS, SYSTEM_VERSION } from "@/shared/api/paths";
import { requestDocumentJson } from "@/shared/api/http";

// ─────────────────────────────────────────────
// System information & runtime statistics
// Matches internal/transport/http/api/v1/system/ types
// ─────────────────────────────────────────────

/** Version info from GET /api/v1/system/version */
export type SystemVersion = {
  version: string;
  buildTime: string;
  commitHash: string;
  goVersion: string;
};

/** Runtime stats from GET /api/v1/system/info */
export type SystemRuntime = {
  goVersion: string;
  numCPU: number;
  numGoroutine: number;
  heapAlloc: number;
  heapSys: number;
  stackInuse: number;
  os: string;
  arch: string;
};

/** Health check from GET /api/v1/system/health */
export type SystemHealth = {
  status: string;
  /** Present only on richer probes (legacy callers); not guaranteed. */
  uptime?: string;
  database?: string;
};

// ─── Zod Schemas ──────────────────────────────────────────────

export const versionSchema = z.object({
  version: z.string(),
  buildTime: z.string(),
  commitHash: z.string(),
  goVersion: z.string(),
});

export const systemInfoSchema = z.object({
  goVersion: z.string(),
  numCPU: z.number(),
  numGoroutine: z.number(),
  heapAlloc: z.number(),
  heapSys: z.number(),
  stackInuse: z.number(),
  os: z.string(),
  arch: z.string(),
});

export const healthSchema = z.object({
  status: z.string(),
  uptime: z.string().optional(),
  database: z.string().optional(),
});

// ─── API functions ──────────────────────────────────────────────

export async function getSystemVersion(): Promise<SystemVersion> {
  const response = await requestDocumentJson(SYSTEM_VERSION);
  return versionSchema.parse(response);
}

export async function getSystemInfo(): Promise<SystemRuntime> {
  const response = await requestDocumentJson(SYSTEM_INFO);
  return systemInfoSchema.parse(response);
}

/** @deprecated Use getSystemInfo instead */
export const getSystemRuntime = getSystemInfo;

export async function getSystemHealth(): Promise<SystemHealth> {
  const response = await requestDocumentJson(SYSTEM_HEALTH);
  return healthSchema.parse(response);
}

// ─── Metrics ──────────────────────────────────────────────

/** Prometheus-compatible metrics in JSON format from GET /api/v1/system/metrics */
export type SystemMetrics = {
  goVersion: string;
  numGoroutine: number;
  numCPU: number;
  heapAllocMB: number;
  heapSysMB: number;
  stackInuseMB: number;
  gcPauseNs: number;
  numGC: number;
  uptimeSeconds: number;
};

export const metricsSchema = z.object({
  goVersion: z.string(),
  numGoroutine: z.number(),
  numCPU: z.number(),
  heapAllocMB: z.number(),
  heapSysMB: z.number(),
  stackInuseMB: z.number(),
  gcPauseNs: z.number(),
  numGC: z.number(),
  uptimeSeconds: z.number(),
});

export async function getSystemMetrics(): Promise<SystemMetrics> {
  const response = await requestDocumentJson(SYSTEM_METRICS);
  return metricsSchema.parse(response);
}
