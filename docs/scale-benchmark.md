# Scale Benchmark Harness

The `v0.9.0-scale-resilience` milestone starts with measurement. This harness
creates deterministic synthetic Kubernetes inventories and benchmarks the
existing KubePreflight pipeline before collector concurrency, timeouts, retry
logic, or Console scaling work is introduced.

This PR does not change detector behavior, findings, report schemas, exit
codes, readiness scoring, or user-facing CLI behavior.

## Scenarios

Synthetic inventories are generated programmatically by
`internal/testutil.GenerateScaleFixture`. No large generated YAML or JSON
fixtures are committed.

| Scenario | Namespaces | Pods | Deployments | StatefulSets | DaemonSets | PDBs | CRDs | Validating webhooks | Nodes | Add-on workloads |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| small | 10 | 100 | 20 | 4 | 3 | 10 | 10 | 4 | 10 | 2 |
| medium | 100 | 1,000 | 200 | 40 | 20 | 100 | 100 | 25 | 100 | 5 |
| large | 1,000 | 10,000 | 800 | 120 | 60 | 300 | 300 | 80 | 500 | 8 |

Each scenario includes mostly control objects plus a bounded set of risky
objects that exercise existing rule families such as drain readiness, PDBs,
CRDs, admission webhooks, and live add-on compatibility. Add-on workloads are
represented as a subset of Deployments with well-known names such as
`metrics-server`, ingress controllers, cert-manager, and external-dns.

Generation is deterministic and index-based. Repeated generation of the same
scenario should produce equivalent inventories, stable finding counts, and
stable finding order.

## Benchmarked Stages

The Go benchmarks cover:

- synthetic fixture generation
- rule evaluation against a `rules.ScanContext`
- `findings.NewReport` construction
- findings JSON rendering
- Markdown report rendering
- HTML report rendering
- scan comparison against a large findings set

Fixture setup is excluded from timed benchmark sections unless the benchmark is
explicitly named `BenchmarkScaleFixtureGeneration`. Other benchmarks build the
fixture before `b.ResetTimer()` and use `b.ReportAllocs()`.

## Commands

Run the developer script:

```bash
scripts/benchmark-scale.sh
```

Run the underlying benchmark command directly:

```bash
go test ./internal/integration \
  -run '^$' \
  -bench 'BenchmarkScale' \
  -benchmem \
  -count=3
```

Optional script settings:

```bash
BENCH_COUNT=5 BENCH_TIME=2s scripts/benchmark-scale.sh
BENCH_PACKAGE=./internal/integration BENCH_REGEX=BenchmarkScaleReport scripts/benchmark-scale.sh
```

When GNU `/usr/bin/time -v` is available, the script also prints elapsed time
and maximum resident set size. If it is not available, benchmarks still run and
the script notes that peak RSS was not captured.

## What Is Measured

The harness measures CPU time and allocations for the in-process rules,
findings, report, and comparison paths. It does not require a Kubernetes
cluster, AWS credentials, or network access.

## What Is Not Measured Yet

This harness does not measure:

- real Kubernetes API server latency
- watch/list pagination behavior
- client-go QPS/burst throttling
- collector timeout or cancellation behavior
- retry/backoff behavior
- browser rendering performance for thousands of Console rows

Fake Kubernetes clients expose action lists, so collector API-call counting is
possible in a future PR. This first harness uses direct synthetic snapshots to
avoid mixing collector transport costs with rule/report baselines.

## No CI Performance Gates Yet

The benchmark output is intentionally not a hard CI gate. Wall-clock timing and
RSS vary by CPU, memory, filesystem, Go version, and container limits. Use the
harness to compare before/after optimization branches on the same machine
rather than to enforce universal thresholds.

When publishing numbers, record:

- commit hash
- CPU model and core count
- RAM and any container memory limit
- OS and architecture
- Go version
- `BENCH_COUNT`, `BENCH_TIME`, and benchmark package/regex
- whether `/usr/bin/time -v` was available

## Comparing Optimization Work

For an optimization branch:

1. Run the benchmark script on current `master`.
2. Run it again on the optimization branch using the same hardware and settings.
3. Compare `ns/op`, `B/op`, `allocs/op`, and peak RSS when available.
4. Confirm ordinary correctness tests still pass and no product semantics
   changed.

## Limitations

Synthetic inventories are useful for deterministic scale pressure, but they are
not a production performance claim. A real cluster can add API-server latency,
authorization overhead, admission side effects, network variability, CRD
storage costs, and provider-specific behavior that direct snapshots do not
model.
