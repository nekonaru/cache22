# Changelog

Semua perubahan penting di Cache22 didokumentasikan di sini. Berkas English adalah sumber kebenaran; `CHANGELOG.md` adalah terjemahan Indonesia.

## v1.0.2 - 2026-11-15

Rilis perbaikan dan dokumentasi: Content Security Policy tidak lagi memblokir render aplikasi, dan dokumentasi repo mengikuti standar dokumentasi proyek.

### Perbaikan bug

- **Web UI.** Atribut inline style melanggar CSP ber-nonce sehingga ditolak browser; sebagian halaman tampil tanpa gaya dan konsol penuh error. Semua gaya dinamis kini berasal dari stylesheet, diverifikasi di DevTools: nol error konsol saat load, create, reveal, dan burn.
- **Fallback tanpa JavaScript.** Atribut inline `style` pada kotak noscript kini memakai class stylesheet.
- **Makefile.** `make test` menjalankan `go vet` lebih dulu agar konsisten dengan CI.

### Peningkatan

- **README.** Kedua berkas disusun ulang sesuai urutan section standar, isi English dan Indonesia selalu sinkron, dan disertai daftar isi.
- **Changelog.** Dipisah menjadi berkas English dan Indonesia dengan konten identik.
- **Metadata repo.** Deskripsi jadi satu kalimat tanpa nama tool, dan topics ditinjau ulang.

## v1.0.1 - 2026-11-15

Patch ketahanan di atas v1.0.0.

### Perbaikan bug

- **CLI.** Flag create yang menggantung (misalnya `-ttl` tanpa nilai) berakhir dengan error yang jelas, bukan panic index out of range.
- **Server.** Response writer CSP nonce kini mengimplementasikan `http.Flusher`, supaya streaming tidak rusak oleh middleware.
- **Web.** Halaman punya fallback `<noscript>` yang menjelaskan JavaScript wajib untuk semua kripto.

## v1.0.0 - 2026-11-15

Rilis publik pertama Cache22: rahasia sekali baca yang terhapus setelah dibaca, dengan enkripsi zero-knowledge di sisi klien.

### Fitur baru

- **Drop sekali baca.** Buat drop (`POST /api/drops`), baca sekali lalu terhapus (`POST /api/drops/{id}/read`), dengan TTL `10m`, `1h`, `24h`, `7d`.
- **Kegagalan seragam.** Id tidak ada, sudah dibaca, kedaluwarsa, atau tidak valid, semuanya mengembalikan `404 {"error":"not_found"}` yang identik.
- **Burn atomik.** Baca dan hapus dalam satu mutex; tes 100 goroutine membuktikan tepat satu pembaca yang menang.
- **Enkripsi zero-knowledge.** AES-256-GCM di browser, kunci hanya di fragmen URL, tidak pernah dikirim ke server.
- **Kunci passphrase.** Wrapping kunci data dengan PBKDF2-HMAC-SHA256 (600.000 iterasi) bersifat opsional; link saja tidak berguna tanpa passphrase.
- **Drop file terenkripsi.** File hingga 2 MB, nama file ikut dalam envelope terenkripsi.
- **UI web ter-embed.** Bilingual (English, Indonesia), gelap dan terang, CSP ketat dengan nonce per request, rahasia dirender via `textContent`.
- **CLI `deadrop`.** Kirim rahasia dari terminal, `deadrop get` baca lalu burn, format wire byte-kompatibel dengan UI web.
- **Rate limit dan sweeper.** Token bucket per IP, interval penyapu 1 menit, `/healthz`, graceful shutdown.
- **Docker image distroless non-root, lisensi MIT, README bilingual, CI.**

### Keterbatasan yang diketahui

- **Penyimpanan in-memory.** Restart menghapus semua drop; penyimpanan persisten ada di roadmap.
- **Perlindungan salinan.** Layanan tidak bisa mencegah penerima menyalin atau memotret rahasia setelah dibuka.

[v1.0.2]: https://github.com/nekonaru/cache22/compare/v1.0.1...v1.0.2
[v1.0.1]: https://github.com/nekonaru/cache22/compare/v1.0.0...v1.0.1
[v1.0.0]: https://github.com/nekonaru/cache22/releases/tag/v1.0.0
