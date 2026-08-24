import type { LogEntry } from "../../types";

const evidenceFields = [
  ["method", "\u65b9\u6cd5"],
  ["error_code", "\u9519\u8bef\u7801"],
  ["reason", "\u539f\u56e0"],
  ["request_id", "\u8bf7\u6c42"],
] as const;

function safeEvidenceValue(value: unknown): string {
  if (typeof value !== "string" && typeof value !== "number" && typeof value !== "boolean") return "";
  const normalized = String(value).replace(/\s+/g, " ").trim();
  if (normalized.length <= 240) return normalized;
  return `${normalized.slice(0, 239)}\u2026`;
}

export function formatLogEvidence(entry: Pick<LogEntry, "fields">): string {
  const fields = entry.fields;
  if (!fields) return "";
  const evidence = evidenceFields.flatMap(([key, label]) => {
    const value = safeEvidenceValue(fields[key]);
    return value ? [`${label}=${value}`] : [];
  }).join(" \u00b7 ");
  if (evidence.length <= 480) return evidence;
  return `${evidence.slice(0, 479)}\u2026`;
}
