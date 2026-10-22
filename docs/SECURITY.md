# Keamanan Phase 2

## Batas kepercayaan

- Aplikasi hanya menerima autentikasi melalui cookie sesi; query token dan bearer admin lama sudah dihapus.
- Token sesi acak hanya dikirim ke browser. Database menyimpan SHA-256 token dan token CSRF.
- Cookie sesi `HttpOnly` dan `SameSite=Strict`; `Secure` wajib pada mode produksi.
- Password memakai Argon2id (64 MiB, 3 iterasi, paralelisme 2).
- Login dibatasi lima percobaan per alamat remote dalam 15 menit. Header proxy tidak dipercaya secara default.
- Credential provider dienkripsi AES-256-GCM dengan nonce acak dan associated data. Master key base64 32 byte hanya berasal dari environment.
- Log tidak boleh memuat token, cookie, password, API key, atau raw Authorization header.

## Operasional

Produksi bersifat fail-closed: `DKO_DATABASE_URL`, master key valid, dan secure cookie wajib. Terminasi TLS dilakukan reverse proxy tepercaya. Rotasi key dan sesi, MFA, trusted-proxy configuration, serta audit append-only eksternal masih merupakan pekerjaan lanjutan.

Migrasi tidak pernah dijalankan otomatis oleh aplikasi. Review SQL, backup database, lalu jalankan secara eksplisit oleh operator. File down migration bersifat destruktif dan tidak boleh dijalankan di produksi tanpa prosedur pemulihan yang disetujui.

Credential provider lama dari `.env` masih dibaca langsung oleh adapter untuk kompatibilitas MVP. Pemindahan credential tersebut ke tabel `credentials` memerlukan UI/command administrasi terpisah sebelum `.env` provider dapat dihentikan.
