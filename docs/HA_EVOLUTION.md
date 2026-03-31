# High Availability Evolution Plan

## Phase 6 Baseline (implemented scaffold)
- Enable role-based node split with deployment-level routing:
  - Control plane: admin/config/API management traffic
  - Relay plane: model forwarding traffic
  - Worker plane: async polling / background jobs
- Add optional tenant audit logging middleware for write operations.
- Add environment controls for trusted proxies and strict CORS.

## Recommended next rollout steps
1. Introduce message queue (Redis Stream / NATS / Kafka) for async task polling replacement.
2. Add read-replica DSN and route read-heavy queries to replicas.
3. Deploy Redis cache-aside for hot metadata (model ratios, channel health, tenant policy).
4. Add HPA with CPU + p95 latency targets.
5. Add SLO dashboard:
   - API p95 / p99
   - Error rate
   - Queue lag
   - DB replica lag
