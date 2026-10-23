# Checkpoint Phase 2

## Selesai

- Skema PostgreSQL versioned untuk account provider, vault credential, snapshot saldo, collector run, user, sesi admin, audit, dan pendapatan terverifikasi.
- Pool pgx dengan timeout, transaksi collector idempotent, retensi saldo terakhir saat collector gagal, dan graceful shutdown HTTP.
- Login Argon2id, cookie session ter-hash, CSRF, rate limit, security headers, serta konfigurasi produksi fail-closed.
- Vault AES-256-GCM versioned dengan nonce acak.
- Unit test auth, vault, HTTP auth/CSRF, dan kontrak respons provider; integration test PostgreSQL opt-in.
- UI admin akun provider dengan CSRF, allowlist, masking credential, enable/disable, audit metadata aman, serta collector multi-account yang membaca vault.
- Migration `000002` mengubah uniqueness menjadi `(provider_slug, account_key)` tanpa menulis ulang migration awal.
- Migration `000003` memperketat idempotency key dan format source hash untuk write baru.
- Retry collector dengan idempotency key sama kini no-op, termasuk bila payload retry berbeda.
- Target integration test divalidasi sebagai database test loopback sebelum koneksi.

## Belum selesai

- MFA, RBAC multi-peran, rotasi master key otomatis, dan audit sink immutable.
- Rekonsiliasi payout untuk mengisi `verified_earnings`.
- Validasi PostgreSQL nyata masih wajib bila `DKO_TEST_DATABASE_URL` belum tersedia; status skip tidak dianggap lulus integrasi.

## Validasi Prompt 002

- Unit test mencakup masking, CSRF mutasi, lookup credential, provider nonaktif, credential invalid, kegagalan repository, isolasi akun, serta respons provider sukses/gagal/malformed/auth.
- Integration test opt-in mencakup idempotency, saldo terakhir saat failure, ciphertext round-trip, isolasi multi-account, dan rollback transaksi.
- Docker daemon tidak tersedia pada checkpoint ini, sehingga migration test dan integration test nyata belum dijalankan.

## Validasi Prompt 003

Lihat `docs/PHASE2-VALIDATION.md`. Pada 23 Oktober 2026 Docker daemon dan target database test tidak tersedia. Unit test, vet, race test, dan diff-check lulus; migration 000001–000003 belum dijalankan dan integration test PostgreSQL berstatus SKIP.

## Review migrasi

- Foreign key credential/snapshot/run tetap `ON DELETE CASCADE`; user audit memakai `ON DELETE SET NULL`.
- Unique snapshot hash dan idempotency collector tetap scoped per akun.
- Migration `000002` menambah indeks akun aktif dan unique `(provider_slug, account_key)`.
- Down migration `000002` fail-closed bila sudah ada lebih dari satu akun per provider, agar rollback tidak menghapus atau menggabungkan data secara diam-diam.
- Aplikasi tidak auto-run migration dan integration test tidak membuat, drop, truncate, atau membersihkan database.

## Prinsip data

`balance_snapshots` hanya menyimpan posisi saldo pada waktu tertentu. `verified_earnings` memerlukan bukti dan verifikasi tersendiri. Keduanya tidak dijumlahkan atau diturunkan satu dari yang lain.

## Validasi Prompt 004

- Root cause adalah lookup slug terhadap map keyed UUID, bukan bukti kehilangan snapshot SQL. `Reading.AccountID` sekarang eksplisit; metadata provider terpisah dan dashboard tetap lookup UUID.
- Perbaikan awal pada working tree dipertahankan. Fetch sukses dan branch awal sama dengan remote; pull tidak diperlukan. Tidak ada push/merge.
- Collector managed menggunakan UUID untuk vault maupun legacy default; fallback environment hanya untuk account key `default`. Akun provider sama tidak digabungkan.
- Regression ditambah untuk multi-account, key existence, latest error vs successful balance, serta identitas hasil collector. Waktu pemeriksaan mengikuti run terbaru.
- Validasi terbaru: test/vet/race seluruh repo FAIL pada setup dependency privat GitHub (autentikasi tidak tersedia). Collector/config/credentials/vault unit tests PASS; PostgreSQL saldo/concurrent idempotency/credential belum dieksekusi. Integration command gagal setup dan target DB tidak tersedia (SKIP eksekusi DB). Detail hasil aktual di `PHASE2-VALIDATION.md`.
- Migration 000001–000003 dilaporkan operator sudah sukses pada PostgreSQL 17.11; tidak dijalankan ulang oleh Codex. Tidak ada migration baru, deploy, DROP/TRUNCATE, atau penghapusan volume.
- Phase 2 belum PASS penuh; rerun PowerShell di dokumen validasi masih diperlukan.
