# Checkpoint Phase 2

## Selesai

- Skema PostgreSQL versioned untuk account provider, vault credential, snapshot saldo, collector run, user, sesi admin, audit, dan pendapatan terverifikasi.
- Pool pgx dengan timeout, transaksi collector idempotent, retensi saldo terakhir saat collector gagal, dan graceful shutdown HTTP.
- Login Argon2id, cookie session ter-hash, CSRF, rate limit, security headers, serta konfigurasi produksi fail-closed.
- Vault AES-256-GCM versioned dengan nonce acak.
- Unit test auth, vault, HTTP auth/CSRF, dan kontrak respons provider; integration test PostgreSQL opt-in.
- UI admin akun provider dengan CSRF, allowlist, masking credential, enable/disable, audit metadata aman, serta collector multi-account yang membaca vault.
- Migration `000002` mengubah uniqueness menjadi `(provider_slug, account_key)` tanpa menulis ulang migration awal.

## Belum selesai

- MFA, RBAC multi-peran, rotasi master key otomatis, dan audit sink immutable.
- Rekonsiliasi payout untuk mengisi `verified_earnings`.
- Validasi PostgreSQL nyata masih wajib bila `DKO_TEST_DATABASE_URL` belum tersedia; status skip tidak dianggap lulus integrasi.

## Validasi Prompt 002

- Unit test mencakup masking, CSRF mutasi, lookup credential, provider nonaktif, credential invalid, kegagalan repository, isolasi akun, serta respons provider sukses/gagal/malformed/auth.
- Integration test opt-in mencakup idempotency, saldo terakhir saat failure, ciphertext round-trip, isolasi multi-account, dan rollback transaksi.
- Docker daemon tidak tersedia pada checkpoint ini, sehingga migration test dan integration test nyata belum dijalankan.

## Review migrasi

- Foreign key credential/snapshot/run tetap `ON DELETE CASCADE`; user audit memakai `ON DELETE SET NULL`.
- Unique snapshot hash dan idempotency collector tetap scoped per akun.
- Migration `000002` menambah indeks akun aktif dan unique `(provider_slug, account_key)`.
- Down migration `000002` fail-closed bila sudah ada lebih dari satu akun per provider, agar rollback tidak menghapus atau menggabungkan data secara diam-diam.
- Aplikasi tidak auto-run migration dan integration test tidak membuat, drop, truncate, atau membersihkan database.

## Prinsip data

`balance_snapshots` hanya menyimpan posisi saldo pada waktu tertentu. `verified_earnings` memerlukan bukti dan verifikasi tersendiri. Keduanya tidak dijumlahkan atau diturunkan satu dari yang lain.
