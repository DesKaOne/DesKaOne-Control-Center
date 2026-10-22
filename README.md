# DesKaOne Control Center 🇮🇩

Dasbor Go berbahasa Indonesia untuk pemantauan saldo multi-provider secara read-only. Phase 2 memakai PostgreSQL 17, sesi admin berbasis cookie, dan fondasi credential vault.

## Persyaratan

- Go 1.23+
- PostgreSQL 17 (Compose opsional)
- `psql` untuk menjalankan migrasi secara eksplisit
- `sqlc` hanya diperlukan bila query generated code akan diregenerasi

## Menjalankan di Windows PowerShell

```powershell
Copy-Item .env.example .env
# Isi POSTGRES_PASSWORD, DKO_DATABASE_URL, DKO_MASTER_KEY, dan password bootstrap.
docker compose up -d postgres

# Review sebelum menjalankan. Jangan jalankan otomatis pada database produksi.
$env:DKO_DATABASE_URL = "postgres://deskaone:ganti-password@127.0.0.1:5439/deskaone_control?sslmode=disable"
Get-Content -Raw migrations/000001_phase2.up.sql | psql -v ON_ERROR_STOP=1 $env:DKO_DATABASE_URL
Get-Content -Raw migrations/000002_provider_accounts_management.up.sql | psql -v ON_ERROR_STOP=1 $env:DKO_DATABASE_URL

go run ./cmd/control-center
```

Setelah admin pertama berhasil dibuat, hapus `DKO_BOOTSTRAP_ADMIN_PASSWORD` dan `DKO_BOOTSTRAP_ADMIN_USERNAME` dari `.env`, lalu restart. Buka http://127.0.0.1:8095/login.

## Pengujian

```powershell
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
go test -race ./...

# Integration test eksplisit pada database test yang skemanya sudah dimigrasikan.
$env:DKO_TEST_DATABASE_URL = "postgres://deskaone:test@127.0.0.1:5439/deskaone_control_test?sslmode=disable"
go test -tags=integration ./internal/database -count=1
```

Unit test tidak mengakses API provider live. Integration test memakai slug unik tetapi tetap harus diarahkan hanya ke database test yang aman dibersihkan operator.

## Model keamanan dan data

- Semua endpoint dashboard/API selain health dan login memerlukan sesi cookie.
- Tidak ada query-token authentication.
- Request provider memakai timeout dan tidak mengikuti redirect secara otomatis.
- Kegagalan collector dicatat tanpa membuat snapshot saldo nol.
- Snapshot saldo terpisah dari tabel pendapatan terverifikasi; saldo tidak dijumlahkan sebagai earnings.
- Migrasi tidak dijalankan otomatis oleh aplikasi.

Lihat [security](docs/SECURITY.md), [arsitektur](docs/ARCHITECTURE.md), dan [checkpoint Phase 2](docs/PHASE2-CHECKPOINT.md).

## Provider

Adapter read-only tersedia untuk ProxyRack, TraffMonetizer, dan Repocket. Admin dapat mengelola beberapa akun melalui `/admin/providers`; secret baru langsung dienkripsi dan form hanya menampilkan status `Tersimpan`.

Variabel credential provider lama tetap didukung hanya untuk akun `default` yang belum memiliki ciphertext. Nilainya tidak pernah otomatis disalin ke PostgreSQL. Setelah credential disimpan lewat UI dan collector terverifikasi, hapus variabel legacy dari `.env`.

Endpoint provider berasal dari allowlist di kode, bukan input form. Endpoint tidak resmi dapat berubah dan harus ditinjau terhadap terms provider sebelum produksi.
