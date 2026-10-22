# Validasi Phase 2 — 23 Oktober 2026

## Status lingkungan

- Branch: `feat/phase2-postgresql-security`.
- Docker CLI tersedia, tetapi daemon tidak tersedia (`docker info` gagal).
- `docker compose config --quiet` gagal karena `.env` lokal belum memiliki konfigurasi Compose Phase 2.
- `psql` tersedia.
- `sqlc` binary tidak tersedia.
- `DKO_TEST_DATABASE_URL` tidak diatur. `.env` hanya memiliki key MVP lama dan nilainya tidak dicetak.

Karena tidak ada target database test yang dapat diverifikasi, migration PostgreSQL dan integration test nyata berstatus **NOT RUN/SKIP**. Tidak ada database dibuat, dihapus, ditruncate, atau dimigrasikan.

## Perbaikan hasil review

- Target integration test sekarang ditolak kecuali nama database mengandung `test` dan host adalah loopback (`localhost`, `127.0.0.1`, atau `::1`).
- Konflik `(provider_account_id, idempotency_key)` sekarang benar-benar no-op. Retry concurrent tidak dapat menyisipkan snapshot kedua dengan payload berbeda.
- Error provider diklasifikasikan menjadi `auth`, `upstream`, `timeout`, `network`, dan `malformed`; error transport disanitasi agar URL tidak ikut tersimpan/log.
- Migration `000003` menambahkan constraint untuk idempotency key non-kosong dan source hash SHA-256 lowercase. Keduanya `NOT VALID`: write baru diperiksa tanpa memindai atau menolak data historis saat migration diterapkan.
- Integration test diperluas untuk concurrency, rollback, multi-account, ciphertext at rest, AAD account binding, dan akun nonaktif.

## Hasil pengujian

| Pemeriksaan | Status | Keterangan |
|---|---|---|
| `go test ./...` | PASS | Seluruh unit test lulus. |
| `go vet ./...` | PASS | Tidak ada temuan. |
| `go test -race ./...` | PASS | Tidak ada data race. |
| `git diff --check` | PASS | Tidak ada whitespace error. |
| Migration 000001 | NOT RUN | Tidak ada database test terverifikasi. |
| Migration 000002 | NOT RUN | Tidak ada database test terverifikasi. |
| Migration 000003 | NOT RUN | Tidak ada database test terverifikasi. |
| Integration PostgreSQL | SKIP | `DKO_TEST_DATABASE_URL` tidak diatur. |
| `sqlc generate` | NOT RUN | Binary `sqlc` tidak tersedia; source dan checked-in generated query direview manual. |

## Prosedur aman Windows PowerShell

Operator harus memastikan target adalah database disposable kosong di PostgreSQL 17 lokal. Jangan gunakan `deskaone_control` atau host remote.

```powershell
# Isi sendiri tanpa menaruh URL atau password ke riwayat bersama.
$env:DKO_TEST_DATABASE_URL = "postgres://USER:PASSWORD@127.0.0.1:5439/deskaone_control_test?sslmode=disable"

# Validasi nama dan host tanpa mencetak password.
$target = [Uri]$env:DKO_TEST_DATABASE_URL
if ($target.Host -notin @("localhost", "127.0.0.1", "::1")) { throw "Host test harus loopback" }
if ($target.AbsolutePath -notmatch "test") { throw "Nama database wajib mengandung test" }

# Periksa versi, database aktif, dan tabel yang sudah ada sebelum tindakan apa pun.
psql $env:DKO_TEST_DATABASE_URL -X -v ON_ERROR_STOP=1 -c "SHOW server_version;"
psql $env:DKO_TEST_DATABASE_URL -X -v ON_ERROR_STOP=1 -c "SELECT current_database(), current_user;"
psql $env:DKO_TEST_DATABASE_URL -X -v ON_ERROR_STOP=1 -c "SELECT schemaname, tablename FROM pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema') ORDER BY 1,2;"
```

Lanjutkan hanya setelah operator memastikan hasil daftar tabel kosong dan database disposable:

```powershell
Get-Content -Raw migrations/000001_phase2.up.sql | psql $env:DKO_TEST_DATABASE_URL -X -v ON_ERROR_STOP=1
Get-Content -Raw migrations/000002_provider_accounts_management.up.sql | psql $env:DKO_TEST_DATABASE_URL -X -v ON_ERROR_STOP=1
Get-Content -Raw migrations/000003_phase2_constraints.up.sql | psql $env:DKO_TEST_DATABASE_URL -X -v ON_ERROR_STOP=1

go test -v -tags=integration ./internal/database -count=1
```

Jangan menjalankan `docker compose down -v`, `DROP DATABASE`, `TRUNCATE`, atau down migration untuk mengulang test pada data yang tidak dipastikan disposable.

## Risiko residual

- **Tinggi untuk readiness produksi:** migration dan integration test belum dijalankan pada PostgreSQL 17 nyata.
- **Sedang:** rate limiter hanya in-memory dan tidak berbagi state antar-instance.
- **Sedang:** fallback credential `.env` masih aktif untuk akun `default` tanpa ciphertext; hapus variabel legacy setelah transisi vault diverifikasi.
- **Sedang:** audit log berada dalam database yang sama dan belum immutable/external.
- **Rendah untuk Phase 2:** MFA, RBAC multi-peran, dan rotasi master key otomatis belum tersedia dan tetap backlog desain.

Phase 2 siap untuk code review, tetapi belum siap produksi sampai seluruh migration dan integration test PostgreSQL nyata lulus.
