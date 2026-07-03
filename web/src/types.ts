import type { Finding, Severity } from "./lib/findings-schema";

export const ALL_SEVERITIES: Severity[] = ["Blocker", "Warning", "Info"];

export interface Filters {
  search: string;
  severities: Severity[];
  confidence: string;
  namespace: string;
}

export const emptyFilters: Filters = { search: "", severities: ALL_SEVERITIES, confidence: "", namespace: "" };

export interface NextAction {
  finding: Finding;
  index: number;
}
