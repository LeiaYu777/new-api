# Testing Optimization

## Problem
Running `go test` in short-lived Docker containers repeatedly downloads dependencies.

## Solution (implemented)
Use project-level persistent caches:
- `.cache/go-mod` for module cache
- `.cache/go-build` for build cache

## Commands
- Docker cached test:
  - `make test-go-docker`
  - or `./scripts/go-test-docker.sh ./middleware/... ./model/...`
- Local cached test:
  - `make test-go-local`
  - or `./scripts/go-test-local.sh ./...`

## Notes
- First run warms the cache (slow).
- Subsequent runs reuse cache and are much faster.
- Set `SKIP_ROOT_EMBED=1` to skip the root package requiring `web/dist` embedding.
