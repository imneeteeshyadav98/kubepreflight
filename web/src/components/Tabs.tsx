export type TabKey = "summary" | "findings" | "actions" | "evidence" | "planner" | "compare";

interface TabsProps {
  active: TabKey;
  onChange: (tab: TabKey) => void;
  findingsCount: number;
  actionsCount: number;
  // hasPlan hides the Planner tab entirely when no upgrade-plan.json was
  // found — the only tab that hides conditionally today; every other tab
  // always renders once any report is loaded.
  hasPlan: boolean;
}

const ORDER: TabKey[] = ["summary", "findings", "actions", "evidence", "planner", "compare"];
const LABELS: Record<TabKey, string> = {
  summary: "Summary",
  findings: "Findings",
  actions: "Next Actions",
  evidence: "Evidence",
  planner: "Upgrade Planner",
  compare: "Compare",
};

// The single-page command-center nav: only one tab's content is mounted
// at a time (see App.tsx), so switching tabs never grows the document —
// each panel scrolls internally instead of the whole page growing long.
export default function Tabs({ active, onChange, findingsCount, actionsCount, hasPlan }: TabsProps) {
  const visible = ORDER.filter((tab) => tab !== "planner" || hasPlan);
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
