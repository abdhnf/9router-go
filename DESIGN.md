# DESIGN — 9router-go

> **Technical Design Document**
> Repo: `luqman-v1/9router-go` · Fork: `abdhnf/9router-go` · Versi: `1.8.17`
> Modul Go: `9router/proxy` · Go `1.27` · CGO-free

---

## 1. Prinsip Desain

1. **Engine-only, dashboard-agnostic.** Go adalah mesin (proxy, SSE, translasi). Next.js adalah UI.
   Keduanya berbagi satu SQLite. Jangan duplikasi logika translasi — cek `internal/translator` dulu.
2. **DB-compatible by construction.** Setiap field baru yang tidak ada di skema masuk ke JSON `data`,
   bukan kolom baru. Ini yang menjaga dashboard tidak pernah rusak.
3. **Registry-driven, bukan hardcode.** Provider, model, alias, kapabilitas, pricing semuanya di
   `internal/providers/`. Menambah provider = menambah entri registry.
4. **Test-first untuk parity.** Setiap port dari upstream disertai test deterministik (mock, tanpa
   network nyata) di `*_test.go`.
5. **Fail-soft, bukan fail-hard.** Model tidak sehat → skip. Error retryable → fallback. Hanya error
   tak terklasifikasi yang dikembalikan ke klien.

---

## 2. Layering

```
cmd/9router-go          → CLI (urfave/cli): serve, version, update, mitm {enable,disable,status}
internal/handlers       → HTTP layer
  ├── chat              → chat/messages/responses/models/combo/fallback (15.685 LOC)
  ├── media             → image/audio/video/embedding/search/scrape/deploy (5.041 LOC)
  ├── oauth             → import token, bulk import (865 LOC)
  └── shared            → helper lintas domain
internal/proxy          → transport, SSE, stall, executor per provider (10.172 LOC)
  ├── executor          → 26 file, signing & protokol non-standar
  └── oauth             → alur OAuth provider
internal/translator     → konversi format dua arah (9.821 LOC)
internal/providers      → registry provider/model/alias/kapabilitas (2.880 LOC)
internal/tokensaver     → RTK / Caveman / Ponytail (1.739 LOC)
internal/db             → SQLite repo layer (2.602 LOC)
internal/usagetracker   → tracker in-flight + ring buffer (878 LOC)
internal/mitm           → CA, DNS, server, handler MITM (1.577 LOC)
internal/headroom       → siklus hidup proxy kompresi (808 LOC)
internal/updater        → self-update (1.018 LOC)
internal/middleware     → auth, logging, max_body, request_id (530 LOC)
```

**Aturan dependensi:** `handlers` → `proxy` → `translator`/`providers`/`db`. Tidak ada import balik.
`translator` tidak boleh tahu HTTP; `db` tidak boleh tahu HTTP.

---

## 3. Alur Request (Chat)

```
POST /v1/chat/completions
  │
  ├─ middleware: request_id → max_body → logging → RequireApiKey (Bearer / X-API-Key / ?key=)
  │
  ├─ resolveModel()
  │    ├─ strip model marker  ([1m] / [1M] → dibuang sebelum resolusi)
  │    ├─ ProviderAliasMap   (ag → antigravity, cc → claude, …)
  │    └─ format  provider/model  atau bare model
  │
  ├─ Is combo? ──ya──▶ handleCombo()
  │                     ├─ applyComboStrategy: sticky | round-robin | fallback | capacity
  │                     └─ fusion → handleFusion() [panel paralel + Judge]
  │
  ├─ DetectRequiredCapabilities(body)   ← vision | pdf | audioInput | audioOutput | tools |
  │                                        search | reasoning | file
  │    └─ reorderByCapabilities(): Tier 0 (punya cap) → Tier 1 (sisanya)
  │
  ├─ Loop model:
  │    ├─ IsProviderHealthy? / IsConnectionModelLocked?  → tidak sehat: skip + warn
  │    └─ tryForwardWithConnection()
  │         ├─ executor.Get(provider)  → executor khusus
  │         ├─ Gemini-native          → forwardGeminiNativeRequest()
  │         └─ default                → forwardRequest() → ForwardOpenAI()
  │
  ├─ Respons:
  │    ├─ stream=true  → handleStreamResponse() + StallReader (timeout 6 mnt, reset per-chunk)
  │    │                 → TranslateOpenAIToClaudeStream() (opsional) → flush
  │    └─ stream=false → handleJSONResponse() → TranslateOpenAIToClaude() → JSON
  │
  └─ Sukses → UnlockConnectionModel() → RecordProviderHealth(reset) → logUsage()
     Error  → ClassifyError() → LockConnectionModel() → fallback model? → ulangi loop
```

**Seed streaming:** `SeedStreamState` menanam model yang diminta klien lewat
`WithRequestedModel` context, sehingga `message_start` menggemakan `combo-wombo`, bukan nama model
provider. Ini mencegah klien salah mengenali model.

---

## 4. Translator

Dua arah, stream & non-stream, tanpa ketergantungan HTTP.

```
internal/translator/
├── request.go             TranslateClaudeToOpenAI, TranslateOpenAIToClaude, sanitasi
├── response.go            TranslateOpenAIToClaude, tool-call aggregation
├── claude_response.go     bentuk respons Anthropic
├── gemini.go              OpenAI ↔ Gemini, NormalizeGeminiContents
├── antigravity.go         cloaking/decloak, prompt stripping, image gen
├── schema.go              cleanGeminiSchema, cleanJSONSchemaForAntigravity
├── tool_schema.go         NormalizeToolSchemasForProvider
├── sanitize.go            SanitizeClaudePassthrough
├── fingerprint.go         OpenCode/Claude desktop fingerprint
├── thought_signature_store.go  penyimpanan thoughtSignature Gemini 3+
├── usage.go               SeedStreamState, akuntansi cached token
└── types.go               tipe bersama
```

### Keputusan Desain Penting

| Masalah | Keputusan |
|---|---|
| Zod-like union di Go | Validasi media diletakkan di level array (`superRefine`-equivalent), bukan per-varian |
| `max_tokens` untuk GPT-5 / o-series | `requiresMaxCompletionTokens` (`/gpt-5|o[134]-/i`) memilih `max_completion_tokens` |
| `thoughtSignature` hilang saat multi-turn | Turn & tool-calling stickiness mengunci ke provider+model sama |
| Gemini tolak `system_instruction.parts[0]` kosong | `StripCompetitivePrompts` membuang part kosong; jika semua kosong → `system_instruction = nil` |
| Gemini tolak `array` tanpa `items` | `cleanGeminiSchema` mengisi `items` default, melebur `prefixItems` |
| Tool name berulang (`get_weather×3`) | Dedup di `ProcessCodexEvent` + `sseToOpenAIJSON` hanya set `name` jika kosong |
| `tool_use.name = call_...` | Jangan fallback ke `tc.ID`; skip tool call tak valid |
| `server_tool_use` ber-ID asing | `SanitizeClaudePassthrough` membuang blok + `tool_result` pasangannya |
| Model marker `[1m]` | `stripModelContextMarker` sebelum `resolveModel` |
| Cached token hanya jalan di antigravity | Parser dual-format (`cache_read_input_tokens` + `prompt_tokens_details.cached_tokens`) |

---

## 5. Lapisan Proxy

### SSE & Stall Detection

- `internal/proxy/sse.go` — penulisan SSE.
- `internal/proxy/sse_scanner.go` — scanner buffer besar (hingga 10 MB) via `proxy.ScanStream`,
  mencegah fragmentasi baris saat payload reasoning terenkripsi >3 KB melewati batas paket TCP.
- `internal/proxy/stall.go` — `StallReader` membungkus body: timeout 6 menit, **reset tiap chunk**,
  tutup koneksi saat stall.
- `internal/proxy/stream_writer.go` — flush terkendali.

### Transport & Egress

- `transport.go` — klien HTTP dengan dukungan proxy pool.
- `proxy.go` — resolusi pool, round-robin.
- Edge relay: header `x-relay-target` / `x-relay-path` (Vercel / Cloudflare / Deno).
- **SSRF guard:** blokir CGNAT `100.64/10`, trailing dot, IPv6 `::ffff:7f00:1` heksadesimal,
  `64:ff9b::`, normalisasi host.

---

## 6. Executor

Registry executor memilih implementasi per provider. Yang perlu signing/protokol khusus:

| Executor | Mekanisme |
|---|---|
| `freebuff.go` | Sesi `POST /api/v1/freebuff/session` (cache TTL 60 mnt, key `token::model`), run via `POST /api/v1/agent-runs`, injeksi marker Buffy, `end_turn` injection, pemetaan root agent `base3-free-*` |
| `qoder.go` | COSY signing: RSA-2048 + AES-128 + MD5 |
| `codebuddy.go` | Streaming CN/INTL, `sseToOpenAIJSON` untuk forced-SSE |
| `trae.go` | Remote agent SOLO |
| `windsurf.go` | gRPC-web |
| `opencode.go` | Responses API, fingerprint desktop, ID sesi `ses_`+12hex+14Base62, ID pesan `msg_`+12hex+14Base62 |
| `claude_messages.go` | Jalur Claude Messages |
| `claude_decloak.go` | Decloak tool pada stream |

**Catatan:** ID sesi/pesan 30 karakter kanonik diperlukan untuk menghindari `403 FreeTierError`
saat merutekan ke `oc/muse-spark-1.3-contributor-free`.

---

## 7. Model Data

### SQLite (WAL)

```sql
apiKeys(id PK, key UNIQUE, name, machineId, isActive, createdAt)
providerConnections(id PK, provider, authType, name, email, priority,
                    isActive, data JSON, createdAt, updatedAt)
kv(scope, key, value, PRIMARY KEY(scope,key))
combos(id PK, name UNIQUE, kind, models, createdAt, updatedAt)
settings(id INTEGER PK CHECK(id=1), data JSON)
providerNodes(id PK, type, name, data JSON, createdAt, updatedAt)
usageHistory(timestamp, provider, model, connectionId, ...)
```

### State di dalam `providerConnections.data` (JSON)

| Field | Fungsi |
|---|---|
| `modelLock_<model>` | Kunci model per-koneksi (isolasi antar koneksi) |
| `backoffLevel` | Tingkat backoff per-koneksi |
| `modelLock` antigravity | Blokir model sampai timestamp reset terverifikasi |

Semua field ini **ditulis dengan bentuk yang sama** seperti dashboard Next.js, sehingga kedua
proses dapat saling membaca tanpa konflik.

### `kv` — konfigurasi yang disinkronkan

`modelAliases`, `customModels`. Perubahan dari dashboard langsung terbaca engine
(`SetCustomModelCaps` refresh live), dan sebaliknya.

---

## 8. Autentikasi & Middleware

```
request_id → max_body → logging → RequireApiKey → handler
```

`ExtractApiKey` urutan: (1) `Authorization: Bearer <key>`, (2) `X-API-Key`, (3) `?key=` query.
Key dicek ke tabel `apiKeys`, ditolak bila `isActive != 1`.
`/health` dan pprof bersifat publik; `/admin/health/reset` **wajib** API key.

`API_KEY_SECRET` default `endpoint-proxy-api-key-secret` — sebaiknya dioverride di produksi.
`INITIAL_PASSWORD` tanpa default (memaksa operator menyetel).

---

## 9. Token Saver

```
internal/tokensaver/
├── compress.go    RTK: kompresi input / tool output
├── injection.go   Penyuntikan prompt Caveman / Ponytail
└── prompts.go     Template per mode (lite | full | ultra | wenyan-ultra)
```

Konfigurasi dibaca dari tabel `settings` SQLite dan disinkronkan otomatis. Injeksi dilakukan di
`chatCore`-equivalent (Go: jalur forward) sebelum request dikirim upstream.

**Uji khusus:** `preserve_numbers_test.go` memastikan angka tidak dirusak kompresi;
`injection_toolresult_test.go` memastikan injeksi tidak merusak blok tool result.

---

## 10. Observabilitas

| Mekanisme | Detail |
|---|---|
| **Usage stream** | `internal/usagetracker`: tracker in-flight + ring buffer recent requests, disiarkan SSE di `/api/usage/stream`. Bentuk item identik dengan dashboard Next.js (`ActiveRequest`, `RecentRequest`) agar animasi topologi langsung jalan |
| **Console logs** | Ring buffer dalam proses + SSE `/translator/console-logs/stream`, keepalive 25 s, event `init`/`line`/`clear` |
| **Tracing** | `/debug/traces` menyajikan latensi p50/p95 per provider+model |
| **Health** | `/health` publik; `/admin/health/reset` terproteksi; `internal/db/health.go` menyimpan state kesehatan provider |

---

## 11. MITM

```
internal/mitm/
├── cert.go      generasi & pemasangan CA
├── dns.go       pemetaan DNS
├── server.go    server MITM
├── manager.go   orkestrasi lifecycle
└── handlers/    antigravity.go, codex.go, copilot.go, cursor.go, kiro.go, base.go
```

CLI: `9router-go mitm enable|disable|status`. Digunakan untuk mencegat lalu lintas klien resmi
(IDE/CLI) dan mengarahkannya ke engine.

---

## 12. Strategi Pengujian

| Lapis | Pendekatan |
|---|---|
| Unit | Test per paket; helper `internal/dbtest` menyediakan skema kanonik + DB sementara |
| Parity | `providers_v065_test.go`, `providers_v069_test.go`, `gemini38_e2e_test.go`, `chat_v065_test.go` memaku perilaku versi upstream tertentu |
| E2E | `*_e2e_test.go` dengan **mock deterministik, tanpa network nyata** |
| Benchmark | `benchmark/` + `benchmark/RESULTS.md`, mock upstream latency 5 ms |
| CI | `.github/workflows/ci.yml`: `go vet ./...` → `go test ./... -v` → build `-ldflags="-s -w"` |

**Aturan kontribusi:** perubahan tidak dianggap selesai sebelum `go test` relevan dijalankan dan
output aslinya ditampilkan. Kegagalan harus dibedakan "pre-existing" vs "regresi" dengan bukti
perbandingan sebelum/sesudah.

---

## 13. Build & Distribusi

```makefile
build:   binary untuk platform saat ini
cross:   cross-compile (CGO-free, tanpa syarat toolchain)
test:    go test ./...
vet:     go vet ./...
bench:   benchmark
docker:  image container
mitm-*:  enable / disable / status
```

- **CGO-free** — `modernc.org/sqlite` (pure Go), bukan `mattn/go-sqlite3`.
- Binary `-ldflags="-s -w"` ≈ 16 MB.
- Dependency minimal: `go-chi/chi/v5`, `modernc.org/sqlite`, `urfave/cli/v2`, `google/uuid`.
- Volume Docker bernama `9router-data` — **jangan pernah diganti nama**; Compose akan membuat volume
  baru yang kosong tanpa error aplikasi, membuat data SQLite tampak hilang.

---

## 14. Utang Teknis yang Diketahui

| Item | Status |
|---|---|
| `handleComboFallback` / `handleMessagesComboFallback` dulu memanggil `tryForwardWithConnection` langsung sehingga `LockConnectionModel` tidak pernah jalan → backoff 429 mati di jalur router | **Sudah diperbaiki** |
| `Quota Percentage Clamping` — nilai upstream >100 atau negatif | **Sudah diperbaiki** (clamp `[0,100]`) |
| Onboarding antigravity: `ONBOARD_MAX_ATTEMPTS` default 2 (turun dari 5), `ONBOARD_RETRY_DELAY_MS` 12 s, backoff eksponensial menghormati context cancellation | **Sudah diperbaiki** |
| Daftar utang aktif selengkapnya | Lihat `TECHNICAL_DEBT.md` |

---

## 15. Batasan Arsitektur (yang harus dihormati)

1. **Satu proses per file SQLite.** Dua proses dengan volume terpisah tidak berbagi state fitness
   proxy pool.
2. **Tanpa dashboard.** Repo ini tidak boleh menumbuhkan UI; itu domain 9Router Next.js.
3. **Tanpa duplikasi translasi.** Semua konversi format lewat `internal/translator`.
4. **Tanpa kolom DB baru** di luar skema kanonik — gunakan JSON `data`.
5. **Tanpa perubahan nama volume Docker** `9router-data`.
