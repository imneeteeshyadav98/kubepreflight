export type TabKey = "summary" | "findings" | "actions" | "evidence" | "planner" | "rollback" | "compare";

interface TabsProps {
  active: TabKey;
  onChange: (tab: TabKey) => void;
  findingsCount: number;
  actionsCount: number;
  // Plan and rollback tabs are hidden unless their companion JSON documents
  // were loaded; every scan-native tab renders once any report is loaded.
  hasPlan: boolean;
  hasRollback: boolean;
}

const ORDER: TabKey[] = ["summary", "findings", "actions", "evidence", "planner", "rollback", "compare"];
const LABELS: Record<TabKey, string> = {
  summary: "Summary",
  findings: "Findings",
  actions: "Next Actions",
  evidence: "Evidence",
  planner: "Upgrade Planner",
  rollback: "Rollback",
  compare: "Compare",
};

// The single-page command-center nav: only one tab's content is mounted
// at a time (see App.tsx), so switching tabs never grows the document —
// each panel scrolls internally instead of the whole page growing long.
export default function Tabs({ active, onChange, findingsCount, actionsCount, hasPlan, hasRollback }: TabsProps) {
  const visible = ORDER.filter((tab) => (tab !== "planner" || hasPlan) && (tab !== "rollback" || hasRollback));
  return (
    <nav className="tab-nav" role="tablist" aria-label="Report sections">
      {visible.map((tab) => (
        <button
          key={tab}
          role="tab"
          type="button"
		  data-tab={tab}
		  id={`tab-${tab}`}
		  aria-controls={`panel-${tab}`}
          aria-selected={active === tab}
          className={`tab-button ${active === tab ? "tab-active" : ""}`}
		  onClick={() => onChange(tab)}
		  onKeyDown={(event) => {
			if (event.key !== "ArrowRight" && event.key !== "ArrowLeft") return;
			const buttons = Array.from(event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>('[role="tab"]') ?? []);
			const index = buttons.indexOf(event.currentTarget);
			const next = event.key === "ArrowRight" ? (index + 1) % buttons.length : (index - 1 + buttons.length) % buttons.length;
			buttons[next]?.focus(); buttons[next]?.click();
		  }}
        >
          {LABELS[tab]}
          {tab === "findings" && <span className="tab-count">{findingsCount}</span>}
          {tab === "actions" && <span className="tab-count">{actionsCount}</span>}
        </button>
      ))}
    </nav>
  );
}
