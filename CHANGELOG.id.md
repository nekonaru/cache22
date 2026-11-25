# Changelog

Semua perubahan penting di Cache22 didokumentasikan di sini. `CHANGELOG.md` (English) adalah sumber kebenaran; berkas ini terjemahan Indonesia.

## v1.5.0 - 2026-11-26

Rilis fitur: webhook read receipt bersignature dan pipeline verifikasi yang lebih ketat.

### Fitur baru

- **Webhook read receipt bersignature.** Flag baru `-notify-secret` / `CACHE22_NOTIFY_SECRET` (nonaktif secara default) menambahkan `X-Cache22-Signature: sha256=<HMAC dari body JSON mentah>` agar penerima bisa memverifikasi setiap receipt.

### Peningkatan

- **Pipeline verifikasi.** Menambahkan pre-commit hook, `scripts/verify.sh` lokal, dan ambang coverage 60 persen di CI; coverage total sekarang 66 persen.
- **Tes.** Menambahkan tes parsing config (coverage 100 persen) dan tes flag CLI (coverage 35 persen).

### Keterbatasan yang diketahui

- **Receipt Telegram tetap tidak bersignature.** Bot API tidak punya permukaan verifikasi penerima; gunakan webhook dengan secret bila keaslian penting.
- **Receipt tetap best-effort.** Kedua kanal tidak menahan respons read dan tidak mencoba ulang setelah gagal.
## v1.4.0 - 2026-11-19

Rilis fitur: notifikasi read receipt di Telegram. Operator yang sudah memakai webhook kini punya opsi kedua yang masuk ke Telegram mereka.

### Fitur baru

- **Telegram read receipts.** Flag baru `-telegram-bot-token` / `CACHE22_TELEGRAM_BOT_TOKEN` dan `-telegram-chat-id` / `CACHE22_TELEGRAM_CHAT_ID` (nonaktif secara default). Setelah read berhasil, server mengirim pesan singkat ke Telegram Bot API di chat yang dikonfigurasi. Pesan hanya memuat metadata: event, kind, sisa baca, waktu baca.
- **Keduanya bisa aktif.** Webhook dan Telegram sama-sama dipicu pada read yang sama ketika keduanya dikonfigurasi; keduanya best-effort dengan timeout klien 2,2 detik dan tidak pernah menahan respons.

### Keterbatasan yang diketahui

- **Receipt tidak bersignature dan best-effort.** Kedua kanal melaporkan ada read; keduanya tidak bisa membuktikan identitas.
- **Token bot adalah kredensial.** `CACHE22_TELEGRAM_BOT_TOKEN` hanya di environment; tidak pernah di-commit, di-log, atau di-echo.
## v1.3.1 - 2026-11-18

Patch ke Blueprint Render: Render mengirim PORT=10000 secara default, jadi servis gratis bakal gagal mengikat port di belakang load balancer Render.

### Perbaikan bug

- **Render Blueprint.** `CACHE22_ADDR` sekarang `:10000` mengikuti default port web service Render; servis listen di situ saat deploy lewat Blueprint, bukan `:8080`.
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

[v1.5.0]: https://github.com/nekonaru/cache22/compare/v1.4.0...v1.5.0
[v1.4.0]: https://github.com/nekonaru/cache22/compare/v1.3.1...v1.4.0
[v1.3.1]: https://github.com/nekonaru/cache22/compare/v1.3.0...v1.3.1
[v1.3.0]: https://github.com/nekonaru/cache22/compare/v1.2.0...v1.3.0
[v1.2.0]: https://github.com/nekonaru/cache22/compare/v1.1.0...v1.2.0
[v1.1.0]: https://github.com/nekonaru/cache22/compare/v1.0.3...v1.1.0
[v1.0.3]: https://github.com/nekonaru/cache22/compare/v1.0.2...v1.0.3
[v1.0.2]: https://github.com/nekonaru/cache22/compare/v1.0.1...v1.0.2
[v1.0.1]: https://github.com/nekonaru/cache22/compare/v1.0.0...v1.0.1
[v1.0.0]: https://github.com/nekonaru/cache22/releases/tag/v1.0.0
