import type { Finding } from "./lib/findings-schema";

export interface Filters {
  search: string;
  severity: string;
  confidence: string;
  namespace: string;
}

export const emptyFilters: Filters = { search: "", severity: "", confidence: "", namespace: "" };

export interface NextAction {
  finding: Finding;
  index: number;
}
