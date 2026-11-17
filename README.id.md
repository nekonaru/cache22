<div align="center">
  <img src="assets/icon.svg" alt="Logo Cache22" width="120" />
  <h1>Cache22: One-Time Secret Sharing</h1>
  <p><b>Bagikan rahasia sekali baca yang hilang setelah dibaca,<br/>terenkripsi sebelum meninggalkan perangkatmu.</b></p>

  <a href="https://github.com/nekonaru/cache22/releases"><img src="https://img.shields.io/github/v/release/nekonaru/cache22?style=flat-square&color=blue" alt="Release" /></a>
  <a href="https://github.com/nekonaru/cache22/actions/workflows/ci.yml"><img src="https://github.com/nekonaru/cache22/actions/workflows/ci.yml/badge.svg?branch=main&style=flat-square" alt="CI" /></a>
  <img src="https://img.shields.io/badge/Go-1.23%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/Platform-Linux%2C%20macOS%2C%20Windows-blue?style=flat-square" alt="Platform" />
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License: MIT" /></a>
  <img src="https://img.shields.io/badge/Status-Active-brightgreen?style=flat-square" alt="Status" />
</div>

<br />

[English](README.md) | **Bahasa Indonesia**

> [!NOTE]
> Cache22 menghapus rahasia setelah dibaca. Server hanya menyimpan ciphertext, dan kunci tidak pernah keluar dari perangkat pengirim.

---

## Daftar Isi

- [Tentang](#tentang)
- [Fitur](#fitur)
- [Kebutuhan](#kebutuhan)
- [Instalasi](#instalasi)
- [Pemakaian](#pemakaian)
- [API](#api)
- [Konfigurasi](#konfigurasi)
- [Struktur Proyek](#struktur-proyek)
- [Pengembangan](#pengembangan)
- [Pengujian](#pengujian)
- [Keamanan](#keamanan)
- [Integrasi Kontinu](#integrasi-kontinu)
- [FAQ](#faq)
- [Penulis](#penulis)
- [Lisensi](#lisensi)

---

## Tentang

**Cache22** membagikan teks atau file kecil sebagai rahasia sekali baca. Kamu menempelnya, mendapat satu link unik, lalu mengirinkannya. Orang yang membuka link melihat isinya sekali; isinya kemudian dihapus permanen dari server. Enkripsi terjadi di perangkat pengirim, jadi layanan ini tidak pernah tahu apa yang kamu bagikan.

> _"One link, one read, then nothing."_

---

## Fitur

### Mengirim

| Fitur | Deskripsi |
|-------|-----------|
| **Link sekali baca** | Tempel teks atau file kecil, dapatkan link unik yang hilang setelah dibaca |
| **Rahasia teks hingga 64 KB** | Tempel langsung di UI |
| **Rahasia file hingga 2 MB** | Nama file ikut terenkripsi bersama file-nya |
| **Kedaluwarsa yang bisa diatur** | 10 menit, 1 jam, 24 jam atau 7 hari |
| **Batas dibuka** | Sekali baca secara default; opsional hingga 1 sampai 10 kali |

### Keamanan

| Fitur | Deskripsi |
|-------|-----------|
| **Kripto zero-knowledge** | AES-256-GCM di browser; kunci tetap di fragmen URL |
| **Kunci passphrase opsional** | PBKDF2-HMAC-SHA256, 600.000 iterasi; link saja tidak berguna tanpa passphrase |
| **Kegagalan seragam** | Id tidak ada, sudah dibaca, kedaluwarsa, atau tidak valid semuanya mengembalikan `404` yang sama |
| **Burn atomik** | Baca dan hapus dalam satu lock; pembaca bersamaan tidak bisa berdua menang |

### Privasi

| Fitur | Deskripsi |
|-------|-----------|
| **Tanpa plaintext di server** | Server hanya menyimpan ciphertext dan meta kecil yang opak |
| **Kunci tidak pernah dikirim** | Fragmen dihapus dari address bar saat halaman dimuat |
| **Render aman** | Rahasia dirender lewat `textContent`; CSP memakai nonce per request dan melarang script inline |
| **Logging minimal** | Log hanya berisi method, route template, status, dan durasi |

### CLI

| Fitur | Deskripsi |
|-------|-----------|
| **CLI `deadrop`** | Pipa rahasia dari terminal dan dapatkan link kembali |
| **Baca lalu burn** | `deadrop get <url>` mendekripsi, mencetak atau menyimpan file, lalu membakar drop |
| **Format kompatibel web** | Link buatan CLI terbuka di UI web, dan sebaliknya |

### Hosting

| Fitur | Deskripsi |
|-------|-----------|
| **Satu binary statis** | Hanya Go standard library; UI tertanam di binary |
| **Docker image** | Distroless, non-root, siap di belakang reverse proxy |
| **Penyimpanan persisten default** | bbolt database; selamat dari restart; `-data -` untuk mode memori sementara |
| **Kompresi gzip** | Halaman dan respons API dikirim sebagai gzip saat klien mendukung (sekitar sepertiga byte) |
| **Webhook read-receipt** | -webhook / CACHE22_WEBHOOK_URL opsional; POST setiap read; anti-SSRF |
| **Rate limiting per IP** | Token bucket bawaan, disetel lewat flag atau env |

### Antarmuka

| Fitur | Deskripsi |
|-------|-----------|
| **UI bilingual** | English dan Indonesia, dideteksi dari browser |
| **Gelap dan terang** | Mengikuti skema warna sistem |
| **Responsif** | Berjalan di ponsel dan desktop |

---

## Kebutuhan

| Dependency | Versi Minimum | Catatan |
|------------|---------------|---------|
| **Go** | 1.23 | Hanya untuk build dari source |
| **Git** | Apa pun | Hanya untuk build dari source |
| **Docker** | 24 | Hanya untuk menjalankan container image |
| **Browser modern** | Dukungan Web Crypto | Chrome, Firefox, Safari, Edge (versi terbaru) |

> [!TIP]
> Kalau kamu hanya ingin memakai Cache22, Go dan Docker tidak perlu. Unduh arsip dari halaman [Releases](https://github.com/nekonaru/cache22/releases) atau `go install` (lihat Instalasi).

### Belum pernah pakai terminal? Mulai dari sini

<details>
<summary><b>Cara membuka terminal</b></summary>

1. Windows: tekan tombol **Windows**, ketik `PowerShell`, lalu tekan **Enter**. Jendela biru terbuka; ketik perintah di sana.
2. macOS: tekan **Command + Space**, ketik `Terminal`, lalu tekan **Return**.
3. Linux: tekan **Ctrl + Alt + T**, atau cari `Terminal` di menu aplikasi.

</details>

---

## Instalasi

### Opsi A: `go install` (termudah)

Membutuhkan Go 1.23 atau lebih baru.

```bash
go install github.com/nekonaru/cache22/cmd/cache22@latest
go install github.com/nekonaru/cache22/cmd/deadrop@latest
cache22
```

Buka <http://localhost:8080>. Arsip prebuilt untuk Linux, macOS, dan Windows juga tersedia di setiap [release](https://github.com/nekonaru/cache22/releases); setiap arsip berisi `cache22`, `deadrop`, README, dan lisensi.

### Opsi B: Build dari source (untuk development)

#### 1. Ambil kode proyek

<details>
<summary><b>Saya sudah punya Git</b></summary>

```bash
git clone https://github.com/nekonaru/cache22.git
cd cache22
```

</details>

<details>
<summary><b>Saya belum punya Git</b></summary>

1. Buka halaman repo di GitHub.
2. Klik tombol hijau **Code**, pilih **Download ZIP**.
3. Ekstrak ZIP dan buka terminal di folder hasil ekstrak.

</details>

#### 2. Pasang dependencies dan verifikasi

```bash
go vet ./...
go test -race ./...
```

> [!IMPORTANT]
> `go vet` tidak boleh melaporkan masalah dan `go test -race ./...` harus lolos sebelum kamu mengandalkan sebuah build.

#### 3. Build dan jalankan

```bash
make build     # bin/cache22 dan bin/deadrop
./bin/cache22  # mendengarkan di :8080
```

### Opsi C: Render.com (satu klik, gratis)

Repo ini menyertakan Blueprint `render.yaml`. Buka https://render.com/deploy?repo=https://github.com/nekonaru/cache22, masuk, lalu setujui blueprint-nya. Render membangun Dockerfile menjadi free web service; `/healthz` dipakai sebagai health check.

Free plan punya filesystem yang ephemeral, jadi servis berjalan dengan `-data -` (penyimpanan in-memory murni). Setelah deploy pertama, set `CACHE22_BASE_URL` ke URL servismu (misalnya `https://cache22-xxxx.onrender.com`) di menu **Environment**, karena link yang dibuat menyertakannya.

> [!NOTE]
> Render free services tidur setelah 15 menit tanpa traffic dan perlu sekitar satu menit untuk bangun pada request berikutnya. Kunjungan pertama setelah idle bisa lambat.

---

## Pemakaian

### Saat pertama dibuka

1. Jalankan `cache22` (atau `make run` untuk development).
2. Buka <http://localhost:8080>.
3. Tempel rahasia kamu, pilih kedaluwarsa, klik **Create link**.
4. Kirim link-nya, plus passphrase secara terpisah kalau kamu mengaktifkannya.
5. Setelah penerima membuka link, rahasia terhapus.

> [!NOTE]
> Link berisi kunci dekripsi di fragmennya (`#...`). Siapa pun yang memegang link lengkap bisa membaca rahasianya sampai dihapus.

### Pemakaian sehari-hari

- **Tempel rahasia**: cukup ketik `token: abc123` di kotaknya.
- **Lampirkan file**: klik drop zone untuk memilih file hingga 2 MB; nama file ikut terenkripsi.
- **Kunci dengan passphrase**: aktifkan toggle, lalu bagikan passphrase lewat kanal lain. Link saja tidak berguna tanpa itu.
- **Batasi dibuka**: ubah "Max opens" dari 1 menjadi 10.
- **Dari terminal**: `echo "db-password: hunter2" | deadrop` mencetak link.
- **Baca lagi**: `deadrop get <url>` mencetak rahasia teks atau menyimpan file dengan izin `0600`.

---

## API

| Metode dan path | Fungsi |
|-----------------|--------|
| `POST /api/drops` | Membuat drop. Body: `{payload, ttl, max_reads?, kind, meta?}`. Mengembalikan `201 {id, url, expires_at}` |
| `GET /d/{id}` | Halaman penerima. Selalu `200`, identik untuk semua id |
| `POST /api/drops/{id}/read` | Mengonsumsi drop. Mengembalikan `200 {payload, kind, meta}` atau `404 {"error":"not_found"}` yang seragam |
| `GET /healthz` | Health check. Mengembalikan `200 ok` |

`payload` adalah base64url dari `iv[12]` ditambah ciphertext AES-256-GCM atas envelope JSON `{"v":1,"kind":"text|file","name"?,"data":base64url}`. `ttl` harus salah satu dari `10m`, `1h`, `24h`, `7d`. `max_reads` default `1`, maksimum `10`. Id acak 128-bit, base64url (22 karakter). Error: `400`, `413`, `429`, `503`.

---

## Konfigurasi

| Flag | Env | Default |
|------|-----|---------|
| `-addr` | `CACHE22_ADDR` | `:8080` |
| `-base-url` | `CACHE22_BASE_URL` | `http://localhost:8080` |
| `-data` | `CACHE22_DATA` | `cache22.db` |
| `-max-text-bytes` | `CACHE22_MAX_TEXT_BYTES` | `65536` |
| `-max-file-bytes` | `CACHE22_MAX_FILE_BYTES` | `2097152` |
| `-max-total-bytes` | `CACHE22_MAX_TOTAL_BYTES` | `67108864` |
| `-rate-per-min` | `CACHE22_RATE_PER_MIN` | `30` |
| `-burst` | `CACHE22_BURST` | `10` |
| `-trust-proxy` | `CACHE22_TRUST_PROXY` | `false` |
| `-webhook` | `CACHE22_WEBHOOK_URL` | _(kosong)_ |

`-trust-proxy` membuat server mempercayai `X-Forwarded-For` untuk rate limiting; aktifkan hanya di belakang reverse proxy yang tepercaya. Semua respons membawa `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, `X-Robots-Tag: noindex`, dan `Content-Security-Policy` ketat dengan nonce per request.

`-webhook` mengirim POST notifikasi read-receipt ke URL tersebut setiap kali drop dibaca. Isinya JSON: `{"event":"drop.read","id":"...","kind":"text","reads_left":0,"read_at":"..."}`. URL divalidasi sebelum setiap pengiriman: host harus publik dan tidak boleh ada alamat hasil resolve yang private, loopback, link-local, atau multicast. Jika tidak, dilewati dengan error log. Isinya tidak pernah memuat ciphertext, kunci, atau payload.

`-data` menunjuk ke file database bbolt; set menjadi `-` untuk memaksa penyimpanan in-memory (tanpa persistensi). Database dibuat dengan izin owner-only (`0600`); isinya hanya ciphertext dan meta terbungkus passphrase, tidak pernah plaintext atau kunci.

---

## Struktur Proyek

```
cache22/
├── cmd/
│   ├── cache22/              # entrypoint server
│   └── deadrop/              # CLI
├── internal/
│   ├── config/               # flag dan env, tervalidasi
│   ├── crypt/                # kripto sisi CLI, cermin browser
│   ├── ratelimit/            # token bucket per IP
│   ├── server/               # handler, header, logging
│   └── store/                # interface Store, implementasi memori dan bbolt
├── web/
│   ├── index.html            # UI satu halaman
│   └── web.go                # go:embed
├── assets/                   # logo, mark, icon, favicon
├── README.md                 # dokumentasi English (sumber kebenaran)
├── README.id.md              # dokumentasi Indonesia
├── CHANGELOG.md              # changelog English
├── CHANGELOG.id.md           # changelog Indonesia
├── Dockerfile                # distroless, non-root
└── Makefile                  # build, test, run, docker
```

---

## Pengembangan

### Mengubah teks UI

1. Edit objek `en` dan `id` di bagian bawah `web/index.html` bersamaan.
2. Jalankan `go build ./...` dan muat ulang halaman.

### Menambah flag atau env var

1. Tambahkan di `internal/config/config.go` beserta default-nya.
2. Pakai dari `internal/server` atau `cmd/cache22`.
3. Dokumentasikan di tabel Konfigurasi di kedua README.

### Rilis

1. Naikkan versi di `CHANGELOG.md` dan `CHANGELOG.id.md`.
2. Tag `vX.Y.Z` di `main` setelah CI hijau.
3. CI membangun arsip; workflow rilis melampirkannya dengan `SHA256SUMS.txt`.

> [!WARNING]
> CI menegakkan `gofmt`. Jangan merapikan alignment manual; jalankan `gofmt -w .` sebelum commit.

---

## Pengujian

Verifikasi sebelum setiap push; keduanya harus bersih:

```bash
go vet ./...
go test -race ./...
```

| Check | Requirement |
|-------|-------------|
| `go vet ./...` | Nol masalah |
| `go test -race ./...` | Semua tes lolos, termasuk tes race 100 goroutine dan test vector format wire JS/Go |

---

## Keamanan

Cache22 dibangun di atas properti-properti terverifikasi ini: enkripsi AES-256-GCM di perangkat pengirim, tidak ada plaintext atau kunci di server maupun di log, respons `404` seragam untuk semua kegagalan baca, burn atomik dalam satu mutex, Content Security Policy ketat ber-nonce, dan token bucket per IP. `gofmt`, `go vet` dan `go test -race` berjalan di setiap push.

> [!NOTE]
> Laporkan kerentanan secara privat sesuai [`SECURITY.md`](SECURITY.md). Jangan buka issue publik untuk itu.

---

## Integrasi Kontinu

| Workflow | Fungsi |
|----------|--------|
| `ci.yml` | `gofmt`, `go vet`, `go test -race`, build server dan CLI, build Docker image |

CI berjalan pada push ke `main`, pada pull request, dan pada tag `v*`.

---

## FAQ

<details>
<summary><b>Bisakah server membaca rahasia saya?</b></summary>

Tidak. Rahasia dienkripsi di browser kamu dengan AES-256-GCM sebelum dikirim. Kunci dekripsi tetap di fragmen URL (`#...`) dan tidak pernah ditransmisikan. Server menyimpan ciphertext dan meta kecil yang opak.
</details>

<details>
<summary><b>Apakah penerima bisa menyimpan salinannya?</b></summary>

Cache22 menghapus rahasia setelah dibaca, tapi tidak bisa mengendalikan apa yang disalin, ditempel, atau difoto penerima.
</details>

<details>
<summary><b>Apakah rahasia selamat dari restart server?</b></summary>

Ya. Data drop disimpan di disk dalam database bbolt (izin owner-only, hanya ciphertext). Jika dijalankan dengan `-data -`, semuanya disimpan di memori dan restart menghapusnya.
</details>

<details>
<summary><b>Kenapa link salah dan drop yang sudah dibaca terlihat sama?</b></summary>

Keduanya mengembalikan respons `404 {"error":"not_found"}` yang byte-nya identik, jadi tidak ada yang bisa tahu apakah suatu id ada, sudah dibaca, atau kedaluwarsa.
</details>

<details>
<summary><b>Kenapa bukan Argon2 untuk passphrase?</b></summary>

Web Crypto API tidak menyediakan Argon2, jadi Cache22 memakai PBKDF2-HMAC-SHA256 dengan 600.000 iterasi dan salt acak per rahasia.
</details>

---

## Penulis

<div align="center">

| [![Nicolas Dwi Dharma](https://github.com/nekonaru.png?size=100)](https://github.com/nekonaru) |
|:---:|
| **Nicolas Dwi Dharma** |
| [github.com/nekonaru](https://github.com/nekonaru) |

</div>

---

## Lisensi

Didistribusikan di bawah **MIT License**. Lihat [`LICENSE`](LICENSE) untuk informasi lebih lanjut.
