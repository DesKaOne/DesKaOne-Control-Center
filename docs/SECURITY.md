# Keamanan Phase 2

## Batas kepercayaan

- Aplikasi hanya menerima autentikasi melalui cookie sesi; query token dan bearer admin lama sudah dihapus.
- Token sesi acak hanya dikirim ke browser. Database menyimpan SHA-256 token dan token CSRF.
- Cookie sesi `HttpOnly` dan `SameSite=Strict`; `Secure` wajib pada mode produksi.
- Password memakai Argon2id (64 MiB, 3 iterasi, paralelisme 2).
- Login dibatasi lima percobaan per alamat remote dalam 15 menit. Header proxy tidak dipercaya secara default.
- Credential provider dienkripsi AES-256-GCM dengan nonce acak dan associated data. Master key base64 32 byte hanya berasal dari environment.
- Log tidak boleh memuat token, cookie, password, API key, atau raw Authorization header.
- UI provider hanya mengembalikan metadata akun dan status `Tersimpan`; ciphertext, nonce, dan plaintext tidak pernah dikirim kembali.
- Provider dipilih dari allowlist dan URL API tidak dapat dimasukkan melalui form, sehingga UI administrasi tidak membuka jalur SSRF.
- Setiap akun memakai UUID akun sebagai associated data AES-GCM untuk mencegah pertukaran ciphertext lintas akun.
- Error collector disanitasi dan diklasifikasikan tanpa menyimpan URL request, header autentikasi, atau credential.
- Integration test fail-closed: hanya menerima database bernama test pada host loopback.

## Operasional

Produksi bersifat fail-closed: `DKO_DATABASE_URL`, master key valid, dan secure cookie wajib. Terminasi TLS dilakukan reverse proxy tepercaya. Rotasi key dan sesi, MFA, trusted-proxy configuration, serta audit append-only eksternal masih merupakan pekerjaan lanjutan.

Migrasi tidak pernah dijalankan otomatis oleh aplikasi. Review SQL, backup database, lalu jalankan secara eksplisit oleh operator. File down migration bersifat destruktif dan tidak boleh dijalankan di produksi tanpa prosedur pemulihan yang disetujui.

Credential provider lama dari `.env` hanya menjadi fallback eksplisit bagi akun `default` tanpa ciphertext. Aplikasi tidak pernah menyalin plaintext itu otomatis. Simpan credential baru melalui `/admin/providers`, verifikasi collector, lalu hapus variabel legacy secara manual.

Audit lokal mencatat actor, aksi, dan ID akun tanpa metadata rahasia. Audit sink immutable, MFA, RBAC multi-peran, dan rotasi master key otomatis tetap merupakan pekerjaan lanjutan.
