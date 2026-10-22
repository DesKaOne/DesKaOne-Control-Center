# ROADMAP

- [x] Go modular starter, server, Bahasa Indonesia UI, health endpoint, JSON API.
- [x] Collector awal ProxyRack, Repocket, TraffMonetizer (belum diuji memakai kredensial nyata).
- [x] `.env` reader, unit test dasar, snapshot error retention.
- [ ] PostgreSQL via pgx/sqlc dan skema migrasi; hapus JSON cache dari mode produksi.
- [ ] Auth session cookie + MFA + RBAC, rate limits dan encrypted credential vault.
- [ ] Tests fixture setiap adapter dan API rate limiting.
- [ ] Collectors CashPilot lainnya serta Wipter, BrigOPS, URnetwork, ProxyBase, ProxyHub.
- [ ] Earnings ledger/payout reconciliation & historical graphs.
- [ ] Node Agent, telemetry, alert, audit immutable.
- [ ] Docker image, environment validation, hardened nginx deployment, monitoring.
