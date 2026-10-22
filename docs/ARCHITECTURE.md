# DesKaOne Control Center Architecture v0.2 — Implementation Checkpoint

Approved baseline v0.1: VPS pusat; read-only; node agent opsional; repository terpisah.

v0.2 expansion: Go-native, antarmuka Bahasa Indonesia, provider registry (CashPilot sebagai referensi, tidak menjadi dependency), PostgreSQL sebagai target produksi, collector modular per provider, wallet/points/usage dipisah, security defaults.

## Status
MVP saat ini: Go stdlib dashboard, adapter ProxyRack/TraffMonetizer/Repocket, JSON API, token gate, polling, snapshot cache atomik, `.env` loader. **Belum PostgreSQL**; JSON cache hanya untuk uji lokal. Belum UI edit konfigurasi, RBAC/MFA, encryption-at-rest, payouts, aggregation revenue, node agent, atau dashboard produksi.

## Policies
- Adapter read-only; tidak mencoba bypass CAPTCHA. Sesi yang diberikan pemilik akun disimpan lokal dan dapat kedaluwarsa.
- API nonresmi berisiko berubah/tidak diizinkan; verifikasi terms dan akses tiap provider sebelum deploy.
- Tidak menganggap error balance sebagai nol. Snapshot terakhir dipertahankan, status error ditampilkan.
- Default loopback; deployment publik perlu reverse proxy TLS, autentikasi kuat, rate limiting, vault credential dan hardening.
- CashPilot tidak ditranslasi/di-copy verbatim; implementasi Go didasarkan pada studi kontrak API dan harus ditinjau untuk lisensi serta kepatuhan.
