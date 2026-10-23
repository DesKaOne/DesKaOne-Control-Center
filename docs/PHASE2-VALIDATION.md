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

## Hasil pengujian historis Prompt 003

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

## Rerun Prompt 004 — Windows PowerShell

Migration 000001–000003 sudah sukses pada PostgreSQL 17.11 `deskaone_control_test` menurut bukti operator di prompt 004. Sesi Codex tidak memverifikasi ulang server tersebut dan **tidak menjalankan migration apa pun**. Jangan mengulang migration untuk rerun test.

Pada PowerShell yang sudah memiliki dependency privat dan `DKO_TEST_DATABASE_URL`:

```powershell
if (-not $env:DKO_TEST_DATABASE_URL) { throw "DKO_TEST_DATABASE_URL belum diatur" }
$target = [Uri]$env:DKO_TEST_DATABASE_URL
if ($target.Host -notin @("localhost", "127.0.0.1", "::1")) { throw "Host test harus loopback" }
if ($target.AbsolutePath -notmatch "test") { throw "Nama database wajib mengandung test" }
go test ./...
go vet ./...
go test -race ./...
go test -v -tags=integration ./internal/database -count=1
git diff --check
```

Jangan mencetak URL/credential, menjalankan DROP/TRUNCATE, down migration, atau menghapus volume.

## Risiko residual

- **Tinggi untuk readiness produksi:** migration dan integration test belum dijalankan pada PostgreSQL 17 nyata.
- **Sedang:** rate limiter hanya in-memory dan tidak berbagi state antar-instance.
- **Sedang:** fallback credential `.env` masih aktif untuk akun `default` tanpa ciphertext; hapus variabel legacy setelah transisi vault diverifikasi.
- **Sedang:** audit log berada dalam database yang sama dan belum immutable/external.
- **Rendah untuk Phase 2:** MFA, RBAC multi-peran, dan rotasi master key otomatis belum tersedia dan tetap backlog desain.

Phase 2 siap untuk code review, tetapi belum siap produksi sampai seluruh migration dan integration test PostgreSQL nyata lulus.

## Hasil aktual Prompt 004 — 23 Oktober 2026

Bagian ini menggantikan klaim readiness historis di atas untuk perubahan terbaru.

- Branch benar `feat/phase2-postgresql-security`; working tree awal **tidak bersih** (empat file berisi perbaikan awal). Perubahan tersebut dipertahankan dan dilengkapi. `git fetch origin` sukses; HEAD sama dengan remote sebelum perubahan (ahead/behind 0/0), sehingga tidak perlu pull. Tidak ada reset/rebase/merge/push.
- Root cause: test mengakses map dengan slug, sedangkan SQL memilih `pa.id::text` (UUID). Missing key menghasilkan zero-value Reading; ini bukan bukti snapshot hilang. Lookup regression kini memakai dua-value lookup.
- Kontrak: `LatestReadings` selalu keyed UUID; `Reading.AccountID` adalah UUID dan `Reading.Provider` metadata slug di collector/repository, nama tampilan pada rows dashboard. Managed `Collector.ID()` selalu UUID baik dari vault maupun fallback legacy. Collector bootstrap `Default()` tetap berslug untuk `EnsureProviders`; hasilnya diselesaikan hanya ke akun `default`. Caller lama tanpa AccountID boleh memberi slug lewat Provider; AccountID eksplisit tidak pernah fallback ke slug.
- Snapshot hash memakai UUID akun; constraint idempotency dan snapshot tetap scoped per akun. Audit transaksi: insert run `ON CONFLICT DO NOTHING RETURNING` berhenti saat konflik; snapshot hanya ditulis saat sukses dan commit bersama run; numeric malformed rollback seluruh transaksi. Belum ada bukti runtime terbaru karena integrasi terhambat.
- Dashboard sebelumnya sudah lookup UUID sehingga mismatch test tidak langsung menghilangkan saldo dashboard. Kini rows tanpa reading juga memiliki AccountID; metadata tidak lagi menyamar sebagai identitas. Status/pesan/waktu pemeriksaan mengikuti run terakhir, saldo/currency tetap snapshot sukses terakhir.
- Fallback credential environment dibatasi akun `default`, sehingga akun tambahan provider sama tidak memakai credential default tanpa sengaja.
- Regression mencakup identitas collector sukses/error, isolasi dua akun dashboard, fallback default, legacy write ke UUID read, retensi saldo saat error beserta pesan/waktu, concurrent retry, serta idempotency key sama lintas akun. Tidak ada migration baru.

| Pemeriksaan terbaru | Status | Bukti |
|---|---|---|
| `go test ./...` | FAIL (setup) | Dependency privat `deskaone-sdk-go-v2` tidak dapat diambil: autentikasi GitHub tidak tersedia. Collector/config/credentials/vault lulus. |
| `go vet ./...` | FAIL (setup) | Terhambat dependency privat yang sama. |
| `go test -race ./...` | FAIL (setup) | Terhambat dependency privat yang sama; collector/config/credentials/vault lulus dengan race detector. |
| `go test -v -tags=integration ./internal/database -count=1` | FAIL (setup); DB SKIP | Gagal memuat dependency sebelum test berjalan; `DKO_TEST_DATABASE_URL` tidak diatur. |
| Saldo/error, concurrent idempotency, credential PostgreSQL | SKIP | Belum dieksekusi pada sesi ini; tidak diklaim PASS. |
| `git diff --check` | PASS | Tidak ada whitespace error. |
| Migration 000001–000003 | Tidak dijalankan ulang | Keberhasilan PostgreSQL 17.11 berasal dari laporan operator, bukan eksekusi sesi ini. |

**Phase 2 belum PASS penuh.** Rerun pada PowerShell dengan dependency privat dan database test yang sudah terkoneksi tetap wajib.
