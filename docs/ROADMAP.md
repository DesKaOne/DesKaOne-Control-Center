# ROADMAP

- [x] Go modular starter, server, Bahasa Indonesia UI, health endpoint, JSON API.
- [x] Collector awal ProxyRack, Repocket, TraffMonetizer (belum diuji memakai kredensial nyata).
- [x] `.env` reader, unit test dasar, snapshot error retention.
- [x] PostgreSQL via pgx/sqlc schema dan migrasi versioned; JSON cache dihapus.
- [x] Auth session cookie, rate limit, CSRF, dan primitive encrypted credential vault.
- [x] Fixture HTTP sukses, gagal, malformed, auth serta integration test PostgreSQL opt-in.
- [ ] MFA, RBAC, UI vault credential, dan rotasi master key.
- [ ] Collectors CashPilot lainnya serta Wipter, BrigOPS, URnetwork, ProxyBase, ProxyHub.
- [ ] Earnings ledger/payout reconciliation & historical graphs.
- [ ] Node Agent, telemetry, alert, audit immutable.
- [ ] Docker image, environment validation, hardened nginx deployment, monitoring.
