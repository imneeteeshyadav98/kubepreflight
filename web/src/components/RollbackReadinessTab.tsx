import {
  eligibilityStatusLabel,
  readinessStatusLabel,
  rollbackDecisionLabel,
  rollbackReasonCodeLabel,
  rollbackStatusClass,
  type RollbackAssessment,
  type RollbackReasonCode,
} from "../lib/rollback-schema";

type RollbackReadinessTabProps = {
  assessment: RollbackAssessment;
};

function windowLabel(assessment: RollbackAssessment): string {
  const minutes = assessment.eligibility.remainingMinutes;
  if (minutes === undefined) return "Unknown";
  const safe = Math.max(0, minutes);
  return `At least ${Math.floor(safe / 60)}h ${safe % 60}m remaining`;
}

// Human title first, raw machine code second (as secondary/technical
// metadata) — matches the "human message before rule ID" pattern already
// used everywhere else in this product (Findings tab, Next Actions,
// report.html finding cards). The raw code stays visible, never hidden,
// since operators grep logs and internal/rollback source by it.
function ReasonCodeList({ reasons }: { reasons: RollbackReasonCode[] }) {
  if (reasons.length === 0) return <p className="comparison-empty">No reason codes recorded.</p>;
  return (
    <ul className="evidence-list reason-code-list">
      {reasons.map((code) => {
        const label = rollbackReasonCodeLabel(code);
        return (
          <li key={code}>
            {label.title}
            {label.explanation && <span className="reason-code-explanation"> — {label.explanation}</span>}
            <code className="reason-code">{code}</code>
          </li>
        );
      })}
    </ul>
  );
}

export default function RollbackReadinessTab({ assessment }: RollbackReadinessTabProps) {
  return (
    <div className="tab-panel rollback-tab" tabIndex={0}>
      <section className={`plan-verdict-banner ${rollbackStatusClass(assessment.recommendation.decision)}`}>
        <p className="eyebrow">Rollback recommendation</p>
        <h2>{rollbackDecisionLabel(assessment.recommendation.decision)}</h2>
        <p>
          Confidence: {assessment.recommendation.confidence}. Evidence is {assessment.evidence.complete ? "complete" : "incomplete"}.
        </p>
      </section>

      <section className="rollback-overview">
        <dl>
          <div>
            <dt>Cluster</dt>
            <dd>{assessment.cluster.name || "Unknown"}</dd>
          </div>
          <div>
            <dt>Current version</dt>
            <dd>{assessment.cluster.currentVersion || "Unknown"}</dd>
          </div>
          <div>
            <dt>Rollback target</dt>
            <dd>{assessment.cluster.rollbackTargetVersion || "Unknown"}</dd>
          </div>
          <div>
            <dt>Eligibility</dt>
            <dd title={assessment.eligibility.status}>{eligibilityStatusLabel(assessment.eligibility.status)}</dd>
          </div>
          <div>
            <dt>Readiness</dt>
            <dd title={assessment.readiness.status}>{readinessStatusLabel(assessment.readiness.status)}</dd>
          </div>
          <div>
            <dt>Rollback window</dt>
            <dd>{windowLabel(assessment)}</dd>
          </div>
        </dl>
      </section>

      <section className="rollback-reasons">
        <p className="eyebrow">Reason codes</p>
        <ReasonCodeList reasons={assessment.recommendation.reasonCodes} />
      </section>

      <section className="rollback-checks">
        {assessment.checks.map((check) => (
          <article key={check.id || check.title} className="rollback-check">
            <div>
              <p className="eyebrow">{check.status}</p>
              <h3>{check.title || check.id}</h3>
            </div>
            {check.reasonCodes.length > 0 && <ReasonCodeList reasons={check.reasonCodes} />}
            {check.evidence.length > 0 && (
              <ul>
                {check.evidence.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            )}
          </article>
        ))}
      </section>
    </div>
  );
}
