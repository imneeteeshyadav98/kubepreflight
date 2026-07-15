import { rollbackDecisionLabel, rollbackStatusClass, type RollbackAssessment } from "../lib/rollback-schema";

type RollbackReadinessTabProps = {
  assessment: RollbackAssessment;
};

function windowLabel(assessment: RollbackAssessment): string {
  const minutes = assessment.eligibility.remainingMinutes;
  if (minutes === undefined) return "Unknown";
  const safe = Math.max(0, minutes);
  return `At least ${Math.floor(safe / 60)}h ${safe % 60}m remaining`;
}

function reasonList(reasons: string[]): string {
  if (reasons.length === 0) return "none";
  return reasons.join(", ");
}

export default function RollbackReadinessTab({ assessment }: RollbackReadinessTabProps) {
  return (
    <div className="tab-panel rollback-tab">
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
            <dd>{assessment.eligibility.status}</dd>
          </div>
          <div>
            <dt>Readiness</dt>
            <dd>{assessment.readiness.status}</dd>
          </div>
          <div>
            <dt>Rollback window</dt>
            <dd>{windowLabel(assessment)}</dd>
          </div>
        </dl>
      </section>

      <section className="rollback-reasons">
        <p className="eyebrow">Reason codes</p>
        <p>{reasonList(assessment.recommendation.reasonCodes)}</p>
      </section>

      <section className="rollback-checks">
        {assessment.checks.map((check) => (
          <article key={check.id || check.title} className="rollback-check">
            <div>
              <p className="eyebrow">{check.status}</p>
              <h3>{check.title || check.id}</h3>
            </div>
            {check.reasonCodes.length > 0 && <p className="rollback-code-line">{reasonList(check.reasonCodes)}</p>}
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
