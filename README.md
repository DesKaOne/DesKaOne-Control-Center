# DesKaOne Control Center 🇮🇩

Prototype Go dashboard berbahasa Indonesia untuk pemantauan passive income multi-provider.

## Jalankan lokal
Requires Go 1.23+.

```bash
cp .env.example .env
# Atur DKO_* credentials pada .env bila ingin menghubungkan provider
# chmod 600 .env on Linux
go test ./...
go run ./cmd/control-center
```
Buka http://127.0.0.1:8095

Saat kosong, dashboard tetap menampilkan penyedia sebagai **belum dikonfigurasi**. Tidak membutuhkan kredensial untuk melakukan test lokal.

Contoh JSON API: http://127.0.0.1:8095/api/v1/providers

## Security
**Jangan expose MVP ke Internet.** Belum ada MFA, login berbasis sesi, rate limiting atau vault terenkripsi. `DKO_ADMIN_TOKEN` menggunakan HTTP bearer sederhana dan query-token fallback (query token berisiko bocor pada browser/proxy logs): gunakan untuk pengujian lokal saja. Jika tanpa token, bind hanya ke loopback. Gunakan credential testing dan jangan commit `.env`. JSON snapshot tidak menyimpan password/token, namun simpan volume secara aman.

## PostgreSQL
`compose.yaml` menyediakan PostgreSQL 17 untuk tahap integrasi lanjutan. **Aplikasi MVP belum menggunakan PostgreSQL**, jadi menjalankan Compose saja tidak memindahkan data dari cache JSON. Referensi skema ada di `docs/DATABASE-DESIGN.sql` dan bukan migrasi siap produksi.

## Provider
- ProxyRack: API key balance snapshot.
- TraffMonetizer: supplied browser session token balance snapshot (expires; user renews manually).
- Repocket: email/password Firebase flow, no token persistence.
- Wipter, BrigOPS, URnetwork, ProxyBase, ProxyHub dan katalog CashPilot lainnya: roadmap.

Tidak ada login otomatis yang mengakali CAPTCHA, dan tidak ada pengujian live akun. Endpoint tak resmi dapat berubah.
