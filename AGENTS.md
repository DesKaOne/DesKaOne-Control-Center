# AGENTS.md
- Go backend, UI Bahasa Indonesia.
- Jangan log token, cookie, API key, password, raw HTTP auth headers.
- Setiap provider menggunakan adapter independen.
- Semua request provider wajib timeout dan tanpa redirect lintas domain otomatis.
- Pisahkan snapshot saldo dan pendapatan terverifikasi.
- Tambah tests untuk respons API sukses, gagal, malformed dan auth.
- Jangan deploy atau menjalankan migrasi sebelum review produksi.
