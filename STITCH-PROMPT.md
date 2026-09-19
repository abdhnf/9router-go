# STITCH-PROMPT — 9Router Dashboard

> Tempel isi dokumen ini ke Google Stitch sebagai prompt perancangan UI.
> Bila Stitch membatasi panjang prompt, pakai **§1–§4 lebih dulu** (fondasi + navigasi + design
> system), lalu lanjutkan dengan **§5 satu per satu** per layar.
>
> Semua label, istilah, dan nilai di dokumen ini diambil dari API nyata
> (`/api/admin/meta`) — bukan karangan. Lihat `DASHBOARD-API.md` untuk kontrak teknisnya.

---

## 1. Ringkasan Produk

Rancang **dashboard admin** untuk **9Router** — sebuah *gateway routing LLM* self-hosted yang
meneruskan permintaan dari klien AI (Claude Code, Codex, OpenCode, Cursor, dan IDE lain) ke
ratusan penyedia model (OpenAI, Anthropic, Gemini, DeepSeek, Groq, OpenRouter, Antigravity, dll).

Dashboard ini dipakai oleh **satu operator teknis** yang mengelola gateway miliknya sendiri.
Penggunanya paham istilah API, provider, API key, dan rate limit. Ia tidak butuh penjelasan
ramah pemula — ia butuh **kepadatan informasi, kejelasan status, dan kecepatan menemukan
masalah**.

Dashboard ini **menggantikan** dashboard lama. Ia menyatu dengan engine Go yang sama.

### Kesan yang harus muncul
Profesional, gelap, padat, presisi. Terasa seperti *control room* untuk infrastruktur — bukan
aplikasi konsumen. Tenang saat sehat, mencolok saat ada masalah.

---

## 2. Prinsip Desain

1. **Status lebih penting daripada dekorasi.** Kesehatan provider, kuota, dan error harus terbaca
   dalam sekejap. Warna dipakai untuk status, bukan hiasan.
2. **Padat tapi bernapas.** Operator memindai banyak baris; beri kepadatan tinggi dengan
   pemisah jelas, bukan ruang kosong berlebih.
3. **Jangan sembunyikan konsekuensi.** Aksi yang mengubah routing harus menunjukkan dampaknya
   sebelum dikonfirmasi.
4. **Data mentah selalu bisa dilihat.** Beberapa nilai disimpan sebagai JSON bebas; operator
   harus bisa membuka tampilan mentahnya.
5. **Gelap sebagai default.** Ini alat yang dipakai lama, sering di samping editor bertema gelap.

---

## 3. Struktur Navigasi

Sidebar kiri tetap, dapat diciutkan. Urutan menu:

| Ikon | Menu | Isi |
|---|---|---|
| ◉ | **Overview** | Statistik langsung, graf topologi, permintaan terbaru |
| ⛁ | **Providers** | Koneksi provider (akun & kredensial) |
| ✦ | **Models** | Katalog model & alias |
| ⧉ | **Combos** | Grup model dengan strategi routing |
| ⌘ | **API Keys** | Kunci akses klien |
| ⇄ | **Proxy Pools** | Pool IP keluar & relay edge |
| ▤ | **Logs** | Konsol langsung |
| ◈ | **CLI Tools** | Status tool CLI terpasang |
| ∿ | **Traces** | Latensi p50/p95 per provider+model |
| ⚙ | **Settings** | Token saver & konfigurasi global |

Header atas: pencarian cepat, indikator status koneksi langsung (**Live** / **Terputus**),
indikator versi, dan tombol tema.

---

## 4. Design System

### Warna (mode gelap — default)

```
Latar utama        #0B0E14   sangat gelap, hampir hitam kebiruan
Permukaan          #131823   kartu & panel
Permukaan naik     #1B2231   hover, baris terpilih, input
Garis              #232B3B   pemisah halus
Teks utama         #E6EAF2
Teks sekunder      #93A0B8
Teks redup         #5C6980

Aksen utama        #4F8DFD   biru — aksi utama, tautan, fokus
Sukses             #34D399   hijau — sehat, aktif, terkirim
Peringatan         #FBBF24   kuning — kuota menipis, backoff
Bahaya             #F87171   merah — gagal, terkunci, nonaktif
Info               #A78BFA   ungu — kombinasi/fusion, info netral
```

Aturan pemakaian warna:
- **Hijau** hanya untuk status benar-benar sehat. Jangan pakai untuk tombol.
- **Kuning** untuk kondisi sementara yang bisa pulih (backoff, kuota menipis, retry).
- **Merah** untuk kegagalan nyata (401/403, koneksi mati, key nonaktif).
- **Ungu** khusus menandai combo bertipe *fusion*.

### Tipografi

- Antarmuka: **Inter** — 13px dasar, 12px untuk label, 15–18px untuk judul panel.
- Angka & data teknis: **JetBrains Mono** — nomor model, ID, token, latensi, kunci API.
- Bobot: 400 isi, 500 label, 600 judul panel. Hindari 700 kecuali angka besar.

### Bentuk & jarak

- Radius: 8px kartu, 6px input & tombol, 999px lencana.
- Jarak dasar 4px. Padding kartu 16–20px. Jarak antar-bagian 24px.
- Bayangan sangat halus; kedalaman terutama dari perbedaan warna permukaan.
- Tinggi baris tabel 40px; tinggi input 34px.

### Komponen bersama

- **Lencana status** — titik 6px + label. Varian: Sehat, Backoff, Terkunci, Nonaktif, Kuota Habis.
- **Baris koneksi** — nama, provider, lencana status, email, prioritas, indikator kuota, menu `⋯`.
- **Indikator kuota** — bar tipis dengan label persen; kuning di bawah 30%, merah di bawah 10%.
- **Cincin model** — visualisasi model yang sedang mengunci sebuah koneksi.
- **Chip strategi** — `sticky`, `round-robin`, `fallback`, `capacity`, `fusion`.
- **Kartu statistik** — angka besar mono + label kecil + tren opsional.
- **Editor JSON** — panel monospace dengan tombol *Salin* dan *Validasi*.
- **Dialog konfirmasi dampak** — menjelaskan akibat aksi sebelum dijalankan.

---

## 5. Layar

### 5.1 Overview

Judul **Overview**, dengan pengalih rentang waktu: 1 jam · 24 jam · 7 hari · 30 hari.

Baris atas — empat kartu statistik:
- **Permintaan** (total pada rentang terpilih)
- **Token** (masuk + keluar, dipisah halus)
- **Tingkat Keberhasilan** (persen, hijau bila >95%)
- **Latensi p50** (milidetik, mono)

Bagian tengah — **graf topologi langsung**, elemen paling penting di halaman ini:
- Node penyedia di satu sisi, node model di sisi lain.
- Garis antar-node menyala dan bergerak saat permintaan sedang berjalan.
- Node yang sedang sibuk berdenyut lembut. Garis memudar setelah permintaan selesai.
- Node error diberi cincin merah; node dalam backoff diberi cincin kuning.
- Ini animasi, bukan gambar statis. Harus terasa hidup namun tidak mengganggu.

Bawah — tabel **Permintaan Terbaru**: waktu, model, provider, token masuk, token keluar, token
cache (bila ada), dan status. Baris baru masuk dari atas dengan animasi halus. Status memakai
warna: sukses hijau, gagal merah, berjalan biru berdenyut.

### 5.2 Providers

Halaman terpenting. Daftar koneksi provider.

Header: kotak pencarian, filter **provider**, filter **status** (Semua / Sehat / Backoff /
Terkunci / Nonaktif), dan tombol **Tambah Koneksi**.

Setiap kartu koneksi menampilkan:
- Nama koneksi (tebal) dan nama provider (mono, sekunder)
- Lencana status
- Email atau identitas akun
- **Prioritas** — angka, bisa diubah langsung
- Indikator kuota
- Baris kecil berisi model yang sedang terkunci, bila ada
- Menu `⋯`: **Uji Koneksi**, **Reset Kesehatan**, **Aktifkan/Nonaktifkan**, **Duplikat**, **Hapus**

Tombol **Tambah Koneksi** membuka wizard:
1. Pilih provider (daftar bergrup: Langganan, API Key, Lokal, Kustom)
2. Pilih jenis autentikasi: **API Key**, **OAuth**, atau **Cookie**
3. Isi kredensial — untuk API Key, satu field dengan tombol tampil/sembunyikan
4. Untuk OAuth: tombol **Hubungkan** yang membuka alur otorisasi, dengan status proses
5. Beri nama koneksi dan prioritas
6. **Simpan**

Panel samping **Detail Koneksi** saat kartu diklik: tab **Umum** (nama, prioritas, aktif,
email), tab **Data** (pasangan kunci–nilai yang bisa diedit), dan tab **JSON Mentah** dengan
peringatan jelas bahwa mengubah data mentah dapat memengaruhi status kunci dan backoff yang
sedang berjalan.

### 5.3 Models

Katalog model yang tersedia lewat gateway.

Header: pencarian, filter **provider**, filter **kemampuan** (Vision, PDF, Audio, Tools,
Reasoning, Pencarian), dan tombol **Sinkronkan Katalog**.

Daftar model, tiap baris: nama model (mono), provider, chip kemampuan, batas konteks, batas
token keluaran. Model dengan kemampuan vision diberi penanda mata kecil.

Tab kedua: **Alias**. Tabel dua kolom — alias dan model tujuan. Contoh: `ag` → `antigravity`,
`cc` → `claude`. Bisa tambah, ubah, hapus. Ini yang dipakai klien saat memanggil model dengan
nama pendek.

Tab ketiga: **Model Kustom**. Model buatan sendiri dengan provider, ID model, batas konteks,
dan sakelar kemampuan.

### 5.4 Combos

Daftar grup model dengan strategi routing.

Setiap kartu combo: nama, chip strategi, dan daftar model berurutan dengan nomor. Untuk strategi
**fusion**, tampilkan dua kelompok terpisah — **Panel** (model yang menjawab paralel) dan
**Juri** (model yang menyintesis jawaban akhir).

Editor combo:
- Nama
- Pemilih **strategi**: Sticky · Round Robin · Fallback · Capacity · Fusion
- Penjelasan singkat di bawah pemilih yang berubah sesuai pilihan, misalnya *"Round robin
  memutar model setiap permintaan."* atau *"Fusion menjalankan beberapa model paralel lalu
  menggabungkan jawabannya."*
- Penyusun daftar model — tambah, hapus, ubah urutan dengan seret
- Untuk Fusion, bagian **Panel** dan **Juri** terpisah
- **Simpan**

### 5.5 API Keys

Tabel kunci akses klien: nama, kunci (disamarkan seperti `sk-••••••••3f2a` dengan tombol salin),
mesin terkait, tanggal dibuat, dan sakelar aktif.

Tombol **Buat Kunci** menghasilkan kunci baru dan menampilkannya sekali dalam panel yang jelas:
*"Salin sekarang — kunci tidak ditampilkan lagi."* dengan tombol salin besar.

Setiap baris punya menu `⋯`: **Ganti Nama**, **Salin Kunci**, **Nonaktifkan**, **Hapus**.
Kunci nonaktif ditampilkan redup dengan coret halus pada nilainya.

### 5.6 Proxy Pools

Daftar pool IP keluar untuk rotasi.

Setiap kartu pool: nama, jenis (HTTP / HTTPS / SOCKS5), jumlah IP aktif dari total, dan lencana
status pengujian. Baris IP individual menampilkan alamat, negara (bila diketahui), latensi, dan
lencana sehat/mati.

Panel **Relay Edge** terpisah dengan kartu untuk **Vercel**, **Cloudflare**, dan **Deno** —
masing-masing dengan status terpasang, URL relay, dan tombol **Pasang** atau **Pasang Ulang**.

### 5.7 Logs

Konsol langsung. Latar paling gelap di seluruh aplikasi.

- Setiap baris: waktu (mono, redup), tingkat, domain dalam tanda kurung siku, lalu pesan.
- Warna tingkat: INFO abu terang, WARN kuning, ERROR merah.
- Kata kunci penting disorot: nama provider, nama model, kode status HTTP.
- Toolbar: **Jeda gulir otomatis** (sakelar), **Bersihkan**, **Saring berdasarkan tingkat**,
  kotak pencarian.
- Saat gulir otomatis aktif, baris baru muncul di bawah dan mendorong ke atas.
- Bila koneksi langsung terputus, tampilkan bilah tipis di atas: *"Koneksi log terputus —
  menghubungkan ulang…"* dengan penghitung mundur.

### 5.8 CLI Tools

Grid kartu, satu per tool CLI: nama, deskripsi singkat, status terpasang, dan versi.

- Terpasang: lencana hijau + versi mono
- Tidak terpasang: lencana abu + tombol **Pasang**

Kartu harus ringkas — halaman ini untuk memindai cepat, bukan membaca.

### 5.9 Traces

Halaman kinerja.

Bagian atas: pemilih rentang waktu dan filter provider.

Tabel per provider+model: jumlah permintaan, **p50**, **p95**, **p99**, tingkat keberhasilan,
dan waktu tunggu rata-rata sebelum token pertama. Nilai p95 yang menonjol diberi warna peringatan.

Sertakan diagram batang horizontal sederhana yang membandingkan p95 antar model, diurutkan dari
yang paling lambat.

### 5.10 Settings

Bagian-bagian, masing-masing dalam kartu dengan judul dan deskripsi satu baris:

**Token Saver**
- **RTK** — sakelar. *"Kompresi masukan dan keluaran tool."*
- **Caveman** — sakelar + pilihan tingkat: `lite` · `full` · `ultra` · `wenyan-ultra`.
  *"Membuat keluaran model lebih ringkas."*
- **Ponytail** — sakelar + pilihan tingkat: `lite` · `full` · `ultra`.
  *"Mendorong kode seminimal mungkin."*

**Headroom** — URL, sakelar sadar-kode, sakelar kompresi, batas waktu (ms), dan indikator status
proses dengan tombol **Mulai** / **Hentikan** / **Mulai Ulang**.

**Pembaruan** — sakelar pembaruan otomatis, versi saat ini, tombol **Periksa Pembaruan**.

**Strategi Provider** — tabel opsional per provider: pool proxy yang dipakai dan mode rotasi
(`none` · `round-robin` · `random` · `sticky`) dengan batas sticky.

**Lanjutan** — akses ke editor JSON pengaturan mentah, dengan peringatan.

---

## 6. Status Kosong, Muat, dan Galat

Rancang keempat keadaan ini untuk setiap daftar.

- **Kosong** — ikon garis tipis, satu kalimat penjelas, satu tombol aksi utama.
  Contoh Providers: *"Belum ada koneksi provider. Tambahkan satu untuk mulai merutekan."*
- **Memuat** — kerangka abu-abu berbentuk baris, bukan pemutar berputar. Jangan menggeser tata
  letak saat data tiba.
- **Galat** — kartu sebaris dengan pesan, kode status, tombol **Coba Lagi**, dan tombol
  **Lihat Detail** yang membuka respons mentah.
- **Koneksi langsung terputus** — bilah tipis di bagian atas area isi, bukan dialog yang
  menghalangi.

---

## 7. Interaksi Penting

- **Konfirmasi berdampak.** Menonaktifkan provider, menghapus koneksi, mengganti strategi combo,
  atau mereset kesehatan harus membuka dialog yang menyebutkan akibatnya secara konkret.
  Contoh: *"Menonaktifkan koneksi ini akan menghentikan 3 combo yang memakainya."*
- **Umpan balik langsung.** Setiap aksi menampilkan notifikasi ringkas di sudut: berhasil
  (hijau) atau gagal (merah) dengan alasan.
- **Tanpa muat ulang halaman.** Semua perubahan tercermin langsung di daftar.
- **Salin sekali klik.** Kunci API, ID koneksi, dan nama model bisa disalin dengan satu klik.
- **Peringatan data mentah.** Saat membuka editor JSON untuk data koneksi, tampilkan catatan:
  *"Data ini menyimpan status kunci dan backoff yang sedang aktif. Ubah dengan hati-hati."*

---

## 8. Persyaratan Responsif

- **≥1280px** — sidebar terbuka, tabel penuh, panel detail di samping kanan.
- **1024–1279px** — sidebar diciutkan menjadi ikon, panel detail menjadi laci yang menutupi.
- **768–1023px** — tabel berubah menjadi daftar kartu bertumpuk; filter masuk ke panel yang
  bisa dibuka.
- **<768px** — satu kolom; sidebar menjadi laci layar penuh; tombol aksi utama menjadi tombol
  mengambang di kanan bawah.

Graf topologi di Overview harus tetap terbaca di semua ukuran — sederhanakan dengan menyembunyikan
label pada layar kecil, bukan mengecilkan semuanya.

---

## 9. Yang Harus Dihindari

- Jangan pakai ilustrasi kartun, maskot, atau emoji besar sebagai ikon.
- Jangan pakai gradien berwarna-warni sebagai latar. Ini alat infrastruktur.
- Jangan sembunyikan informasi teknis demi kesederhanaan.
- Jangan pakai tombol besar bulat atau gaya aplikasi konsumen.
- Jangan gunakan lebih dari satu warna aksen dominan dalam satu layar.
- Jangan buat animasi yang berjalan terus-menerus tanpa alasan.
- Jangan pakai istilah pemasaran. Pakai istilah teknis yang tepat.

---

## 10. Contoh Isi

```
Providers
──────────────────────────────────────────────────────────────
● Utama              antigravity · oauth         [Sehat]
  budi@example.com                    Prioritas 1   ▓▓▓▓▓░ 82%

● Cadangan           openai · api-key             [Backoff 2m]
  cs@example.com                      Prioritas 2   ▓▓░░░░ 41%

● Lama               deepseek · api-key           [Nonaktif]
  dev@example.com                     Prioritas 9   —
```

```
Combos
──────────────────────────────────────────────────────────────
combo-wombo          [fusion]
  1. cc/claude-sonnet-4-6      panel
  2. ag/gemini-3.8-flash-high  panel
  3. cx/gpt-5.6                juri
```
