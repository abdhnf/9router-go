# PRD — 9router-go

> **Product Requirements Document**
> Repo: `luqman-v1/9router-go` · Fork: `abdhnf/9router-go` · Versi dianalisis: `1.8.17`
> Dihasilkan dari pembacaan langsung source code (252 file `.go`, ±58.738 LOC, 117 file test).

---

## 1. Ringkasan Produk

**9router-go** adalah **mesin proxy LLM berperforma tinggi berbasis Go** yang menggantikan lapisan
routing `/v1/*` milik 9Router (yang aslinya monolit Next.js). Produk ini **headless** — tidak punya
UI sendiri — dan dirancang hidup berdampingan dengan dashboard Next.js milik 9Router, berbagi satu
file SQLite yang sama.

**Satu kalimat:** *"Engine proxy LLM berkecepatan tinggi yang drop-in kompatibel dengan dashboard 9Router yang sudah ada."*

### Masalah yang Diselesaikan

| Masalah di Next.js | Dampak | Solusi Go |
|---|---|---|
| Throughput terbatas ±505 RPS | Bottleneck saat banyak agen/IDE klien | 5.920 RPS (**11,7x**) |
| RSS 270 MB | Boros di VPS kecil / edge | 42,5 MB (**6,4x** lebih ringan) |
| Startup 3–5 detik | Buruk untuk restart cepat / autoscale | <100 ms (**30–50x**) |
| Butuh `node_modules` ±200 MB | Distribusi berat | Binary tunggal 16 MB |
| Single-threaded event loop | Latensi naik tajam di concurrency tinggi | Avg 11 ms @ c=100 vs 108 ms (**9,8x**) |

---

## 2. Target Pengguna

1. **Operator gateway LLM pribadi/tim** — menjalankan 9Router di VPS, ingin throughput lebih tinggi
   tanpa membuang dashboard yang sudah terpasang.
2. **Pengguna CLI coding agent** — Claude Code, Codex, OpenCode, Cursor, Windsurf, Qoder, Trae,
   Kiro, Zed — yang butuh endpoint OpenAI/Anthropic-compatible.
3. **Maintainer/Integrator** — ingin meng-port fitur dari upstream `decolua/9router` secara 1:1.

---

## 3. Tujuan & Non-Tujuan

### Tujuan (In-Scope)

- **G1 — Parity engine 100%** dengan `decolua/9router` (saat ini tersinkron `v0.5.75`–`v0.5.81`
  plus port PR #3973, #3981, #3968).
- **G2 — Zero-migration adoption.** Baca/tulis SQLite yang sama (`~/.9router/db/data.sqlite`, WAL).
- **G3 — Multi-format bidirectional.** OpenAI ↔ Anthropic Claude Messages ↔ Google Gemini, stream & non-stream.
- **G4 — Routing cerdas.** Combo strategy, auto-capability-switch, stickiness multi-turn.
- **G5 — Ketahanan anti-ban** untuk provider yang memeriksa identitas klien (Antigravity, Claude OAuth).
- **G6 — Distribusi ringan.** CGO-free, cross-compile, binary tunggal.

### Non-Tujuan (Out-of-Scope)

- ❌ **Tidak menyediakan dashboard/UI web.** Manajemen provider, API key, combo, dan visualisasi
  tetap milik Next.js 9Router. Repo ini hanya menyediakan endpoint pendukung agar dashboard hidup
  (SSE usage, console log, CLI tools status).
- ❌ **Tidak menyimpan state routing di memori lintas proses.** Satu proses per file SQLite.
- ❌ **Tidak menjadi SaaS multi-tenant.** Ini gateway lokal/self-hosted.

---

## 4. Fitur Fungsional

### F1 — Chat & Messages Multi-Format

| Endpoint | Fungsi |
|---|---|
| `POST /v1/chat/completions` | Chat format OpenAI |
| `POST /v1/messages` | Chat format Anthropic Claude |
| `POST /v1/messages/count_tokens` | Hitung token |
| `POST /v1/responses` | Responses API (opencode/muse-spark) |
| `POST /v1/responses/compact` | Compaction |
| `POST /api/chat` | Kompatibilitas Ollama |

Dukungan: streaming SSE, non-streaming JSON, tool-calling, multimodal (vision/audio/pdf),
`thoughtSignature` backfill untuk Gemini 3+, serta **stall detection** (timeout 6 menit,
reset per-chunk).

### F2 — Registry Provider & Model

- **±135 provider** terdaftar (LLM, image, TTS/STT, music, embedding, search, scrape).
- Alias pendek: `ag` → `antigravity`, `cc` → `claude`, `cx` → `codex`, `cu` → `cursor`, `dc` → `deepseek`, dll.
- `GET /v1/models` + `GET /v1/models/{kind}` (`image`/`tts`/`web`) + lookup catch-all `GET /v1/models/*`
  (contoh `cc/claude-sonnet-4-6`).
- `GET /v1/models/info` mengekspos `context_length`, `max_completion_tokens`, `max_input_tokens`,
  `max_output_tokens` (snake_case).
- Custom model via `kv.customModels` + `SetCustomModelCaps` (refresh live).

### F3 — Combo Strategy (Router)

| Strategi | Perilaku |
|---|---|
| `sticky` | Rotasi setelah N pemakaian beruntun (`consecutiveUseCount`) |
| `round-robin` | Rotasi tiap request (`rrIdx`) |
| `fallback` / `capacity` | Urutan asli, failover saat error |
| `fusion` | Fan-out paralel ke panel model + **Judge** menyintesis jawaban |

**Auto-capability-switch:** `DetectRequiredCapabilities()` memindai isi request dan menaikkan model
berkemampuan cocok ke depan — `vision`, `pdf`, `audioInput`, `audioOutput`, `tools`, `search`,
`reasoning`, `file`.

**Fusion lifecycle:** StragglerGrace 8 s, HardTimeout 90 s, quorum `MinPanel=2`;
0 jawaban → `503`, 1 jawaban → fallback single model, ≥2 → Judge.

### F4 — Ketahanan & Anti-Ban

- **Error classification** berbasis teks + status code, dengan exponential backoff.
- **Per-connection model lock** disimpan sebagai `modelLock_<model>` di JSON
  `providerConnections.data` — kompatibel dengan dashboard.
- **Reactive 401 refresh:** auto-refresh OAuth token saat 401, retry sekali, baru fallback.
- **Antigravity Tool Cloaking:** klien tool di-cloak dengan suffix `_ide`, disuntik 21 decoy tool IDE
  resmi (`run_command`, `replace_file_content`, `grep_search`, `list_dir`, …), riwayat
  `functionCall`/`functionResponse` disinkronkan, lalu di-uncloak saat respons.
- **Anti-Competitive Prompt Stripping:** menghapus identitas agen pihak ketiga (mis.
  *"You are a Claude agent…"* atau identitas Hermes Agent) dari `system_instruction` untuk mencegah
  **429 synthetic quota** / **403 FreeTierError**.
- **Antigravity quota strike-breaker:** 3× 429 berturut pada `connection|model` dalam 60 s →
  `CACHE_BLOCK` 15 menit alih-alih loop 300 s.
- **Claude OAuth cloaking** + `SanitizeClaudePassthrough` (buang `server_tool_use` ber-ID asing).

### F5 — Token Saver

| Saver | Fungsi | Mode |
|---|---|---|
| **RTK** | Kompresi input / tool output | on/off |
| **Caveman** | Output ringkas (terse) | `lite`, `full`, `ultra`, `wenyan-ultra` |
| **Ponytail** | Bias kode minimal (YAGNI) | `lite`, `full`, `ultra` |

Semua dikonfigurasi dari tabel `settings` SQLite dan disinkronkan otomatis
(`RTK_ENABLED`, `CAVEMAN_ENABLED`, `PONYTAIL_ENABLED`).

### F6 — Media & Web Tools

`/embeddings`, `/images/generations`, `/audio/speech`, `/audio/voices`, `/audio/transcriptions`,
`/videos/generations`, `/videos/edits`, `/videos/extensions`, `/videos/{id}`,
`/search`, `/scrape`, `/web/fetch`.

### F7 — Egress & Proxy Pool

- Round-robin IP rotation via pool HTTP/HTTPS/SOCKS5 aktif.
- **Edge relay** Vercel / Cloudflare / Deno (`x-relay-target`, `x-relay-path`).
- **No-auth provider strategy** (`settings.providerStrategies`) untuk provider free-tier.
- **SSRF hardening:** blokir CGNAT `100.64/10`, trailing dot, IPv6 `::ffff:7f00:1` hex,
  `64:ff9b::`, plus `normalizeHost`.

### F8 — Endpoint Pendukung Dashboard

| Endpoint | Kegunaan di Dashboard |
|---|---|
| `GET /api/usage/stream` | SSE in-flight request → animasi pulse & marching-ants pada graf topologi |
| `GET /api/usage/stats` | Statistik usage |
| `GET /translator/console-logs` + `/stream` | "Monitor Console Log" live (keepalive 25 s) |
| `GET /cli-tools/all-statuses` | Status batch tool CLI terpasang |
| `POST /admin/health/reset` | Reset health state (dilindungi API key) |
| `GET /debug/traces` | Latensi p50/p95 per provider+model |
| `POST /headroom/*` | Siklus hidup proxy kompresi Headroom |
| `POST /proxy-pools/{vercel,deno,cloudflare}-deploy` | Deploy relay edge |

### F9 — Executor Khusus Provider

Executor kustom dengan signing/protokol non-standar:

- **`freebuff`** — Codebuff/Buffy. `POST /api/v1/freebuff/session` (token TTL 60 menit),
  `POST /api/v1/agent-runs`, injeksi system marker Buffy, `end_turn` tool injection,
  pemetaan root agent per model (`base3-free-*`).
- **`qoder`** — COSY signing (RSA-2048 + AES-128 + MD5).
- **`codebuddy`** CN/INTL — streaming + `sseToOpenAIJSON`.
- **`trae`** — remote agent SOLO.
- **`windsurf`** — gRPC-web.
- **`opencode` / `opencode-go`** — Responses API, fingerprint desktop, ID sesi/pesan kanonik.
- **`claude`** — cloaking OAuth + dedup `call_`.

### F10 — MITM & Updater

- **MITM** (`internal/mitm`): CA cert, DNS, server, handler untuk `antigravity`, `codex`,
  `copilot`, `cursor`, `kiro`. Perintah: `9router-go mitm {enable,disable,status}`.
- **Updater**: cek versi, update otomatis, toggle auto-update; endpoint
  `/api/version/check`, `/api/version/update`, `/api/version/auto-update`.

---

## 5. Kebutuhan Non-Fungsional

| Aspek | Target |
|---|---|
| Throughput | ≥5.000 RPS non-stream @ c=100 |
| Latensi | ≤15 ms avg @ c=100 |
| Memori | ≤50 MB RSS |
| Startup | <200 ms |
| Distribusi | Binary tunggal, CGO-free, cross-compile |
| DB | SQLite WAL, non-blocking, 1 proses per file |
| Port default | `20128` |
| Data dir | `$DATA_DIR` → default `~/.9router` (`%APPDATA%/9router` di Windows) |
| Test | 117 file `*_test.go`; CI jalankan `go vet` + `go test ./...` + build |

### Konfigurasi Environment

`PORT`, `DATA_DIR`, `DB_PATH`, `JWT_SECRET`, `INITIAL_PASSWORD`, `API_KEY_SECRET`,
`MACHINE_ID_SALT`, `RTK_ENABLED`, `CAVEMAN_ENABLED`, `PONYTAIL_ENABLED`.

> `INITIAL_PASSWORD` sengaja **tanpa default** — operator wajib menyetel eksplisit agar tidak ada
> password bawaan yang diketahui publik.

### Autentikasi

`Authorization: Bearer <key>`, fallback header `X-API-Key`, atau query `?key=`. Key divalidasi
terhadap tabel `apiKeys` + cek `isActive`.

---

## 6. Kompatibilitas Database

Skema SQLite **1:1** dengan dashboard Next.js:

| Tabel | Isi |
|---|---|
| `apiKeys` | `id`, `key`, `name`, `machineId`, `isActive`, `createdAt` |
| `providerConnections` | `id`, `provider`, `authType`, `name`, `email`, `priority`, `isActive`, `data` (JSON), `createdAt`, `updatedAt` |
| `kv` | `(scope, key) → value` — `modelAliases`, `customModels` |
| `combos` | `id`, `name`, `kind`, `models`, `createdAt`, `updatedAt` |
| `settings` | `id=1`, `data` (JSON) |
| `providerNodes` | Provider OpenAI/Anthropic-compatible kustom (UUID suffix) |
| `usageHistory` | `timestamp`, `provider`, `model`, `connectionId`, … |

---

## 7. Metrik Keberhasilan

1. Dashboard Next.js yang sudah ada dapat membaca/menulis DB tanpa migrasi. ✅ *terverifikasi via skema*
2. Semua provider & modalitas (chat, image, video, TTS/STT, music, search, fetch) berfungsi. ✅
3. Tidak ada regresi parity vs upstream pada test suite. ✅ *117 file test, CI hijau*
4. Adopsi: operator dapat menukar engine tanpa mengubah konfigurasi klien.

---

## 8. Risiko & Mitigasi

| Risiko | Tingkat | Mitigasi |
|---|---|---|
| Drift dari upstream `decolua/9router` | Tinggi | Sinkronisasi versi eksplisit + catatan port PR di `CHANGELOG.md` |
| Perubahan skema DB oleh dashboard | Sedang | Skema beku, semua field baru masuk JSON `data` |
| Provider memblokir pola request | Tinggi | Cloaking, prompt stripping, proxy pool, strike-breaker |
| Race dua proses pada satu SQLite | Sedang | Dokumentasi tegas: satu proses per file; WAL |
| Kredensial bocor di log | Sedang | Redaksi key di middleware logging |

---

## 9. Roadmap Terkait

- Metrik analitik visual mendalam pada dashboard web (lihat `ROADMAP.md`).
- Utang teknis aktif dicatat di `TECHNICAL_DEBT.md`.

---

## 10. Definisi "Selesai" untuk Perubahan Apa Pun

Mengacu aturan perilaku di `CLAUDE.md`:

1. Tidak ada klaim tanpa bukti (output test, diff, atau verifikasi reproducible).
2. Skeptis terhadap hasil sendiri — pastikan test menguji hal yang benar.
3. Dilarang memfabrikasi laporan.
4. Bedakan "pre-existing failure" dari "regresi" dengan **membandingkan hasil test sebelum & sesudah**.
5. Laporkan apa adanya, termasuk kaveat dan kegagalan yang tak teratasi.
6. Jalankan test relevan setelah perubahan, tampilkan output asli.
7. Jangan pernah mengganti nama volume Docker `9router-data`.
