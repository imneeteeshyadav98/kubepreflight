export type TabKey = "summary" | "findings" | "actions" | "evidence";

interface TabsProps {
  active: TabKey;
  onChange: (tab: TabKey) => void;
  findingsCount: number;
  actionsCount: number;
}

const ORDER: TabKey[] = ["summary", "findings", "actions", "evidence"];
const LABELS: Record<TabKey, string> = { summary: "Summary", findings: "Findings", actions: "Next Actions", evidence: "Evidence" };

// The single-page command-center nav: only one tab's content is mounted
// at a time (see App.tsx), so switching tabs never grows the document —
// each panel scrolls internally instead of the whole page growing long.
export default function Tabs({ active, onChange, findingsCount, actionsCount }: TabsProps) {
  return (
    <nav className="tab-nav" role="tablist" aria-label="Report sections">
      {ORDER.map((tab) => (
        <button
          key={tab}
          role="tab"
          type="button"
          data-tab={tab}
          aria-selected={active === tab}
          className={`tab-button ${active === tab ? "tab-active" : ""}`}
          onClick={() => onChange(tab)}
        >
          {LABELS[tab]}
          {tab === "findings" && <span className="tab-count">{findingsCount}</span>}
          {tab === "actions" && <span className="tab-count">{actionsCount}</span>}
        </button>
      ))}
    </nav>
  );
}
