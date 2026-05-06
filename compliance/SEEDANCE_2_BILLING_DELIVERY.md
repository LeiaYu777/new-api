# Seedance 2.0 Billing Customer Delivery Checklist

This checklist is for the `codex/seedance-billing-plan` branch.

## 1. Delivery Modes

### AGPL SaaS / Source-Available Delivery

Use this mode when the modified NewAPI service is offered to customers or network users under AGPL-3.0 obligations.

Required artifacts:

1. Exact running source archive for the deployed commit.
2. `LICENSE` and upstream copyright notices.
3. Change log with commit IDs.
4. Build and deployment instructions.
5. SBOM for the delivered release.
6. Seedance 2.0 billing plan, acceptance runbook, monitoring runbook, evidence collector, and evidence verifier.
7. Remaining tasks or known limitations.

### Closed-Source Commercial Delivery

Use this mode only after obtaining commercial authorization from the upstream rightsholder.

Required records:

1. Commercial authorization contract.
2. Invoice or payment proof.
3. Authorized version scope, customer scope, duration, and deployment scope.
4. Internal copy of modified source and SBOM for audit.
5. Customer-facing release notes and operational runbook.

## 2. Package Command

Generate the delivery package from a clean worktree:

```bash
scripts/package-seedance-delivery.sh
```

Generate package and SBOM:

```bash
GENERATE_SBOM=true scripts/package-seedance-delivery.sh
```

Use a custom release version:

```bash
VERSION=2026-05-06-seedance-billing scripts/package-seedance-delivery.sh
```

The output is written to:

```text
compliance/releases/<version>/
```

## 3. Seedance-Specific Files To Include

1. `docs/SEEDANCE_2_BILLING_PLAN.md`
2. `docs/SEEDANCE_2_BILLING_ACCEPTANCE.md`
3. `docs/SEEDANCE_2_BILLING_REMAINING_TASKS.md`
4. `docs/SEEDANCE_2_MONITORING.md`
5. `scripts/seedance-billing-smoke.sh`
6. `scripts/seedance-billing-preflight.sh`
7. `scripts/seedance-billing-collect-evidence.sh`
8. `scripts/seedance-billing-verify-evidence.sh`
9. `scripts/package-seedance-delivery.sh`
10. `deploy/observability/seedance-billing-prometheus.yml`
11. `deploy/observability/seedance-billing-alert-rules.yml`
12. `deploy/observability/seedance-billing-grafana-dashboard.json`
13. `.env.example`

## 4. Release Gate

Do not deliver to production customers until these items are complete:

1. Customer Volcengine Seedance 2.0 channel is configured and tested.
2. Real model ID and request/response schema are confirmed.
3. Wallet and subscription billing acceptance scenarios pass.
4. Payment/recharge path is verified in the customer's payment provider.
5. Task polling worker is enabled and refund behavior is verified.
6. Material URL allowlist or object storage relay is configured.
7. AGPL source package or commercial authorization record is available.

## 5. Evidence To Archive

1. Delivery package path and checksum.
2. SBOM path and checksum.
3. Smoke test terminal output.
4. Evidence collector output under `compliance/evidence/seedance-<timestamp>/`.
5. Evidence verifier terminal output.
6. Billing console screenshots.
7. Exported billing CSV.
8. Customer price configuration screenshots.
9. Prometheus/Grafana target and dashboard screenshots.
10. Commercial authorization proof, if using closed-source delivery.
