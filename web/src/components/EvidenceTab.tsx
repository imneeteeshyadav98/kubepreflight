import { compareFindings, findingResourceLabel, priorityPillClass, type Finding, type Report } from "../lib/findings-schema";

interface EvidenceTabProps {
  report: Report;
  selected?: Finding | null;
}

// Every finding's resource identity and fingerprint — cross-reference by
// fingerprint for waivers/dedup. Mirrors report.html's Evidence Appendix.
// Hidden behind its own tab (was rendered below everything else on the
// long-document page) so it doesn't add to page length until requested.
export default function EvidenceTab({ report, selected }: EvidenceTabProps) {
  return (
    <div className="tab-panel evidence-tab" id="evidence-appendix">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Raw identity data</p>
          <h2>Evidence appendix</h2>
        </div>
        <span>{report.findings.length} findings</span>
      </div>
      <div className="table-wrap">
        <table className="appendix">
          <thead>
            <tr>
              <th>Priority</th>
              <th>Rule ID</th>
              <th>Severity</th>
              <th>Confidence</th>
              <th>Resource</th>
              <th>Fingerprint</th>
            </tr>
          </thead>
          <tbody>
            {[...report.findings].sort(compareFindings).map((finding) => (
              <tr key={finding.fingerprint} className={selected?.fingerprint === finding.fingerprint ? "row-selected" : undefined}>
                <td>
                  {finding.priority && (
                    <span className={`priority-pill ${priorityPillClass(finding.priority)}`} title={finding.priorityReason}>
                      {finding.priority}
                    </span>
                  )}
                </td>
                <td>{finding.ruleId}</td>
                <td>{finding.severity}</td>
                <td>{finding.confidence}</td>
                <td>{findingResourceLabel(finding)}</td>
                <td className="fingerprint">{finding.fingerprint}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
