# Release Checklist (Phase 8)

1. Run security checks:
   - `npm audit --omit=dev` (frontend)
   - `go list -m -u all` + `govulncheck ./...` (backend)
2. Run tests:
   - `make test-go-docker`
   - frontend build / smoke test
3. Generate SBOM:
   - `make sbom`
4. Update compliance artifacts:
   - `compliance/AGPL_COMPLIANCE.md`
   - release changelog
5. Package delivery:
   - source bundle + license notices + SBOM + deployment manifests
