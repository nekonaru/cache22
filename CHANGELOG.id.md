# Changelog

Semua perubahan penting di Cache22 didokumentasikan di sini. `CHANGELOG.md` (English) adalah sumber kebenaran; berkas ini terjemahan Indonesia.

## v1.3.0 - 2026-11-18

Rilis deployment: repo sekarang menyertakan Blueprint Render.com sehingga Cache22 bisa dideploy satu klik.

### Peningkatan

- **Render Blueprint.** `render.yaml` membangun Dockerfile yang ada menjadi free web service dengan `/healthz` dipakai sebagai health check. README dan README.id.md mendokumentasikan catatan ephemeral (free filesystem) dan cara menset `CACHE22_BASE_URL` setelah deploy pertama.
- **Dokumentasi.** Section instalasi kini memuat Opsi A, B dan C dengan konten identik di kedua bahasa.

## v1.2.0 - 2026-11-17

Rilis fitur: notifikasi read receipt opsional bagi operator yang ingin tahu kapan sebuah drop dibaca.

### Fitur baru

- **Webhook read receipt.** Flag baru `-webhook` / `CACHE22_WEBHOOK_URL` (nonaktif secara default): setelah read berhasil, server mengirim POST JSON (`drop.read`, id, kind, reads_left, read_at) ke URL yang dikonfigurasi. Pengiriman best-effort dengan timeout 3 detik; kegagalan dicatat tanpa id.
- **Penjaga SSRF pada webhook.** Validasi endpoint menolak skema non-http(s) dan host mana pun yang resolve ke alamat private, loopback, link-local, CGNAT, multicast, atau unspecified, serta semua redirect. Tidak ada ciphertext, kunci, atau plaintext di payload.

### Keterbatasan yang diketahui

- **Receipt satu arah dan tidak terautentikasi.** URL webhook dikendalikan operator tetapi payload tidak ditandatangani; perlakukan endpoint sebagai untrusted untuk pembuktian, bukan pembuktian identitas.
- **Perlindungan salinan.** Layanan tidak bisa mencegah penerima menyalin atau memotret plaintext setelah reveal.

## v1.1.0 - 2026-11-15

Penyimpanan persisten secara default: drop kini selamat dari restart lewat file database bbolt (default `cache22.db`, izin owner-only), menyelesaikan item roadmap penyimpanan persisten.

### Peningkatan

- **Penyimpanan.** Flag baru `-data` / `CACHE22_DATA` memilih file bbolt persisten (default `cache22.db`); `-` memaksa mode in-memory seperti sebelumnya. `Consume` sekarang berjalan di dalam satu transaksi tulis bbolt. Docker image menset `CACHE22_DATA=/data/cache22.db` dan menyediakan `VOLUME ["/data"]` yang dimiliki user non-root.
- **Dockerfile.** Kini menyalin `go.sum` dan membuat direktori `/data` dengan ownership non-root; `.gitignore` mencakup `cache22.db`.

### Keterbatasan yang diketahui

- **Restart tidak menghapus drop** (kecuali `-data -` diset eksplisit).
- **Link hidup dengan `#fragment` adalah kapabilitas baca** sampai drop dibakar; kunci passphrase ada untuk ini.
- **Cache22 tidak bisa mencegah penerima** menyalin atau memotret plaintext setelah reveal.

## v1.0.3 - 2026-11-15

Rilis performa dan hardening: respons kini ter-gzip ke sekitar sepertiga ukuran aslinya, dan alur reveal dengan passphrase tidak lagi kehilangan rahasia sebelum passphrase dimasukkan.

### Perbaikan bug

- **Web UI.** Untuk drop dengan passphrase, klik pertama sebelumnya mengonsumsi drop tapi pengguna tidak bisa mengulang passphrase saat typo; payload kini ditahan antar klik sehingga passphrase bisa diulang tanpa refetch. Error menyesatkan untuk read yang kena rate limit sekarang jadi toast, bukan pesan palsu "sudah dibaca".

### Peningkatan

- **Gzip.** Respons HTML dan JSON dikirim dengan kompresi gzip saat klien mendukung (10,3 KB wire untuk halaman 31 KB, Vary: Accept-Encoding).
- **Header keamanan.** Menambahkan Cross-Origin-Opener-Policy, Cross-Origin-Resource-Policy, dan Permissions-Policy di setiap respons. HSTS dikirim hanya di belakang -trust-proxy.

## v1.0.2 - 2026-11-15

Rilis perbaikan dan dokumentasi: Content Security Policy tidak lagi memblokir halaman, dan dokumentasi repo mengikuti standar dokumentasi proyek.

### Perbaikan bug

- **Web UI.** Atribut inline style melanggar CSP ber-nonce sehingga ditolak browser; sebagian halaman tampil tanpa gaya dan konsol penuh error. Semua gaya dinamis kini berasal dari stylesheet, diverifikasi di DevTools: nol error konsol saat load, create, reveal, dan burn.
- **Fallback tanpa JavaScript.** Atribut inline `style` pada kotak noscript kini memakai class stylesheet.
- **Makefile.** `make test` menjalankan `go vet` lebih dulu agar konsisten dengan CI.

### Peningkatan

- **README.** Kedua berkas disusun ulang sesuai urutan section standar dengan section identik di kedua bahasa, dan disertai daftar isi.
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

[v1.3.0]: https://github.com/nekonaru/cache22/compare/v1.2.0...v1.3.0
[v1.2.0]: https://github.com/nekonaru/cache22/compare/v1.1.0...v1.2.0
[v1.1.0]: https://github.com/nekonaru/cache22/compare/v1.0.3...v1.1.0
[v1.0.3]: https://github.com/nekonaru/cache22/compare/v1.0.2...v1.0.3
[v1.0.2]: https://github.com/nekonaru/cache22/compare/v1.0.1...v1.0.2
[v1.0.1]: https://github.com/nekonaru/cache22/compare/v1.0.0...v1.0.1
[v1.0.0]: https://github.com/nekonaru/cache22/releases/tag/v1.0.0
