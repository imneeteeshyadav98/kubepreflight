import { findingResourceLabel, type Report } from "../lib/findings-schema";

interface EvidenceTabProps {
  report: Report;
}

// Every finding's resource identity and fingerprint — cross-reference by
// fingerprint for waivers/dedup. Mirrors report.html's Evidence Appendix.
// Hidden behind its own tab (was rendered below everything else on the
// long-document page) so it doesn't add to page length until requested.
export default function EvidenceTab({ report }: EvidenceTabProps) {
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
              <th>Rule ID</th>
              <th>Severity</th>
              <th>Confidence</th>
              <th>Resource</th>
              <th>Fingerprint</th>
            </tr>
          </thead>
          <tbody>
            {report.findings.map((finding) => (
              <tr key={finding.fingerprint}>
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
