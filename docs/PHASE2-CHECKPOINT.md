# Checkpoint Phase 2

## Selesai

- Skema PostgreSQL versioned untuk account provider, vault credential, snapshot saldo, collector run, user, sesi admin, audit, dan pendapatan terverifikasi.
- Pool pgx dengan timeout, transaksi collector idempotent, retensi saldo terakhir saat collector gagal, dan graceful shutdown HTTP.
- Login Argon2id, cookie session ter-hash, CSRF, rate limit, security headers, serta konfigurasi produksi fail-closed.
- Vault AES-256-GCM versioned dengan nonce acak.
- Unit test auth, vault, HTTP auth/CSRF, dan kontrak respons provider; integration test PostgreSQL opt-in.

## Belum selesai

- UI/command pengelolaan credential vault dan migrasi credential provider dari environment.
- MFA, RBAC multi-peran, rotasi master key otomatis, dan audit sink immutable.
- Rekonsiliasi payout untuk mengisi `verified_earnings`.

## Prinsip data

`balance_snapshots` hanya menyimpan posisi saldo pada waktu tertentu. `verified_earnings` memerlukan bukti dan verifikasi tersendiri. Keduanya tidak dijumlahkan atau diturunkan satu dari yang lain.
