# DASHBOARD — Rancangan UI Manajemen Menyatu

> **Keputusan arsitektur:** dashboard ini **MENGGANTIKAN** dashboard 9Router Next.js, bukan
> berdampingan. Konsekuensinya: engine Go harus menjadi **satu-satunya penulis** SQLite, dan
> lapisan manajemennya harus lengkap.
>
> Turunan dari `PRD.md` (bagian 4, 6, 10) dan `DESIGN.md` (bagian 2, 7, 15).

---

## 1. Temuan yang Menentukan Seluruh Rancangan

Saya memeriksa `internal/handlers/router.go` dan mencari endpoint manajemen:

```
/apiKeys              route_count=0
/providerConnections  route_count=0
/combos               route_count=0
/settings             route_count=0
/kv                   route_count=0
/providerNodes        route_count=0
```

**Engine Go saat ini tidak punya satu pun endpoint manajemen.**

Dashboard Next.js upstream tidak memakai API untuk mengelola data — ia membuka
`~/.9router/db/data.sqlite` langsung dari sisi server Next.js. Itu sebabnya
`ARCHITECTURE.md` menyebut *"share SQLite"*, bukan *"call the API"*.

**Karena kita menggantikan dashboard itu, gap ini wajib ditutup di dalam engine Go.**

### Yang sudah tersedia (dipakai apa adanya)

`internal/db/repos.go` sudah punya pembacaan:

```
GetProviderConnections(provider, activeOnly)   GetProviderConnectionByID(id)
GetApiKeyByKey(key)                            GetCombos() / GetComboByName / GetComboById
GetSettings()                                  GetModelAliases() / GetModelAlias
GetCustomModels()                              GetProviderNodeByID / GetProviderNodePrefixMap
GetProxyPool(id)                               GetConnectionBackoffLevel
IsConnectionModelLocked                        GetUsageDaily
```

Yang tulis masih tipis: `CreateProviderConnection`, `InsertProxyPool`, `SetAutoUpdate`,
`SetProviderStrategy`. **Gap utamanya bukan data, tapi operasi tulis + lapisan HTTP.**

---

## 2. Prinsip

1. **Engine adalah satu-satunya penulis DB.** Tidak ada proses kedua. Ini menjaga batasan
   `DESIGN.md` §15 butir 1.
2. **Nol DTO baru.** Pakai struct `internal/models` apa adanya. Dashboard dan engine tidak boleh
   punya dua tafsir atas field yang sama.
3. **JSON blob tetap opaque kecuali field yang dikenal.** Lihat §5 — ini bagian paling berbahaya.
4. **Binary tetap tunggal.** UI di-serve dari `embed.FS`, sesuai NFR `PRD.md` §5 (16 MB, tanpa
   runtime tambahan).
5. **Read-heavy dulu, write lengkap menyusul.** Tapi karena menggantikan, write tidak boleh
   setengah-setengah saat rilis.

---

## 3. Lapisan yang Dibangun

```
internal/handlers/admin/     ← BARU: CRUD manajemen
internal/middleware/admin.go ← BARU: gerbang admin (RequireApiKey saja tidak cukup)
web/                         ← BARU: SPA (build statis, di-embed)
```

### 3.1 Middleware admin

`RequireApiKey` saat ini hanya memeriksa `apiKeys.isActive` — **tidak ada konsep role**.
Menambah CRUD di belakangnya berarti setiap API key menjadi admin penuh.

Pilihan, dan saya sarankan yang pertama:

- **A (rekomendasi):** tambah kolom/field `role` pada `apiKeys` **di dalam JSON `data`**, bukan
  kolom baru — konsisten dengan aturan `DESIGN.md` §1 butir 2 ("field baru masuk JSON, bukan kolom
  baru"). Karena `apiKeys` tidak punya kolom `data`, alternatifnya pakai tabel `kv` scope `admin`.
- **B:** env `ADMIN_API_KEY` terpisah. Cepat, tapi tidak bisa multi-admin.

Saya pilih **A via `kv` scope `admin`** supaya skema tidak berubah sama sekali.

### 3.2 Kontrak endpoint

Semua di bawah `/api/admin`, dilindungi `RequireApiKey` + `RequireAdmin`.

| Resource | Endpoint | Catatan |
|---|---|---|
| Connections | `GET /api/admin/connections` | `?provider=&activeOnly=` → `GetProviderConnections` |
| | `GET /api/admin/connections/{id}` | |
| | `POST /api/admin/connections` | **Merge** `data`, bukan replace (§5) |
| | `PATCH /api/admin/connections/{id}` | Partial; `data` di-merge |
| | `DELETE /api/admin/connections/{id}` | |
| | `POST /api/admin/connections/{id}/health/reset` | pakai `ResetConnectionHealthState` |
| API Keys | `GET/POST/PATCH/DELETE /api/admin/api-keys` | `PATCH` untuk `isActive` |
| Combos | `GET/POST/PATCH/DELETE /api/admin/combos` | `strategy` masuk ke JSON `models` (§4) |
| Settings | `GET /api/admin/settings` | |
| | `PUT /api/admin/settings` | merge, bukan replace |
| KV | `GET/PUT/DELETE /api/admin/kv/{scope}/{key}` | `modelAliases`, `customModels` |
| Provider Nodes | `GET/POST/PATCH/DELETE /api/admin/provider-nodes` | |
| Proxy Pools | `GET/POST/PATCH/DELETE /api/admin/proxy-pools` | |
| Usage | `GET /api/admin/usage/history` | `?from=&to=&provider=&model=` |
| Health | `POST /api/admin/health/reset` | sudah ada, perlu dipindah ke gerbang admin |

**Sudah ada, jangan dibangun ulang:** `/models`, `/v1/models`, `/usage/stream`,
`/usage/stats`, `/translator/console-logs(/stream)`, `/cli-tools/all-statuses`,
`/debug/traces`, `/headroom/*`, `/proxy-pools/*-deploy`, `/api/oauth/{provider}/import`,
`/api/version/*`.

---

## 4. Jebakan Data yang Wajib Ditangani

### 4.1 `combos.strategy` tidak ada di skema

`models.Combo` punya field `Strategy`, tetapi skema kanonik di `internal/dbtest/dbtest.go`
hanya mendefinisikan:

```sql
combos(id PK, name UNIQUE, kind, models, createdAt, updatedAt)
```

**Tidak ada kolom `strategy`.** Artinya strategy hidup di dalam JSON `models`.

**Implikasi:** form combo di UI tidak boleh punya input terpisah yang menulis kolom — ia harus
menulis ke dalam `models` JSON. Salah di sini = combo tersimpan tapi routing mengabaikan strategy,
tanpa error apa pun.

### 4.2 `providerConnections.data` menyimpan state runtime

Field yang **sedang dipakai engine saat berjalan** dan bersembunyi di dalam blob `data`:

| Field | Fungsi | Sumber |
|---|---|---|
| `modelLock_<model>` | Kunci model per-koneksi | `LockConnectionModel` / `UnlockConnectionModel` |
| `backoffLevel` | Tingkat backoff per-koneksi | `GetConnectionBackoffLevel` |
| lock antigravity | Blokir model sampai reset terverifikasi | `antigravity_quota.go` |

**Aturan mutlak untuk editor koneksi:**

> `PATCH /api/admin/connections/{id}` **wajib merge** field yang dikirim ke dalam `data` yang
> sudah ada. **Dilarang replace utuh.**

Kalau replace utuh, kamu menghapus semua `modelLock_*` dan `backoffLevel` yang sedang aktif.
Gejalanya: setelah admin menyimpan perubahan kecil di halaman Providers, semua koneksi langsung
menembak provider yang sedang di-rate-limit, dan `429` banjir. Tidak ada error, tidak ada log —
hanya perilaku yang tiba-tiba buruk.

### 4.3 Bentuk payload SSE sudah dipaku

Dari `internal/usagetracker/tracker.go`:

```go
ActiveRequest { model, provider, account, count }
RecentRequest { timestamp, model, provider, promptTokens, completionTokens,
                cachedTokens?, status }
StreamPayload { activeRequests, recentRequests, errorProvider, pending }
PendingState  { byModel, byAccount }
```

Klien **wajib** memakai bentuk ini. Bikin DTO sendiri = animasi topologi tidak akan pernah cocok,
dan `pending.byAccount` (map bersarang) akan salah dirender.

---

## 5. Peta Halaman → Data

| # | Halaman | Sumber | Status endpoint |
|---|---|---|---|
| 1 | **Overview / Topology** | `/api/usage/stream` (SSE) + `/api/usage/stats` | ✅ ada |
| 2 | **Providers & Connections** | `/api/admin/connections` + `/api/oauth/{p}/import` | ⚠️ CRUD baru |
| 3 | **API Keys** | `/api/admin/api-keys` | ⚠️ CRUD baru |
| 4 | **Combos** | `/api/admin/combos` + `GET /v1/models` | ⚠️ CRUD baru |
| 5 | **Models & Aliases** | `GET /v1/models`, `/api/admin/kv/modelAliases` | ⚠️ KV tulis baru |
| 6 | **Settings** (token saver) | `/api/admin/settings` + `/headroom/status` | ⚠️ settings tulis baru |
| 7 | **Proxy Pools** | `/api/admin/proxy-pools` + `/proxy-pools/*-deploy` | ⚠️ CRUD baru |
| 8 | **Monitor Console** | `/translator/console-logs/stream` (SSE) | ✅ ada |
| 9 | **CLI Tools** | `/cli-tools/all-statuses` | ✅ ada |
| 10 | **Traces** | `/debug/traces` | ✅ ada |
| 11 | **Usage History** | `/api/admin/usage/history` | ⚠️ baru (pakai `GetUsageDaily`) |

**4 dari 11 halaman sudah siap.** Sisanya fokus di 5 halaman manajemen.

---

## 6. Tiga Pola UI yang Tidak Boleh Dilupakan

1. **SSE ganda bersamaan.** `/api/usage/stream` dan `/translator/console-logs/stream` hidup
   paralel dengan keepalive 25 detik. `EventSource` polos akan mati diam-diam — wajib reconnect
   + exponential backoff, dan indikator status koneksi yang terlihat.

2. **Editor JSON untuk `data` dan `models`.** Karena kedua blob itu opaque, UI butuh mode
   lanjutan (raw JSON) di samping form. Tapi default harus form, dengan peringatan eksplisit
   bahwa menyimpan akan mengganti isi — bukan menambah.

3. **Peringatan konsekuensi.** Aksi yang mengubah routing (`isActive`, `priority`, strategy combo,
   lock reset) sebaiknya menampilkan dampak sebelum konfirmasi, karena efeknya terasa di semua
   klien LLM yang terhubung.

---

## 7. Urutan Pengerjaan

**Fase 1 — Fondasi (wajib lebih dulu)**
1. Middleware `RequireAdmin` (`kv` scope `admin`).
2. Satu resource end-to-end: **connections**. Repo → handler → route → test.
   Test pakai `internal/dbtest` (skema kanonik + DB sementara, deterministik, tanpa network).
3. Uji khusus: **PATCH merge** pada `data` — pastikan `modelLock_*` dan `backoffLevel` selamat.

**Fase 2 — Resource sisanya**
`api-keys`, `combos` (hati-hati §4.1), `settings`, `kv`, `provider-nodes`, `proxy-pools`,
`usage/history`.

**Fase 3 — SPA**
Serve dari `embed.FS` agar binary tetap tunggal. Mulai dari halaman yang endpoint-nya sudah ada
(Overview, Console, CLI Tools, Traces) supaya ada hasil terlihat lebih cepat.

**Fase 4 — Ganti total**
Hapus ketergantungan pada dashboard Next.js. Pastikan tidak ada proses lain yang membuka
`data.sqlite`.

---

## 8. Definisi Selesai

Mengacu aturan `CLAUDE.md` (lihat `PRD.md` §10):

1. Setiap endpoint punya test; output asli ditampilkan.
2. Uji regresi PATCH-merge (§4.2) lulus — ini yang paling mudah rusak.
3. `go vet ./...` + `go test ./...` hijau sebelum dan sesudah.
4. Tidak ada kolom DB baru di luar skema kanonik.
5. Tidak ada proses kedua yang menulis `data.sqlite`.
6. Binary tetap tunggal (`embed.FS`, bukan sidecar).

---

## 9. Keputusan Terbuka

| # | Pertanyaan | Dampak |
|---|---|---|
| 1 | Role admin disimpan di `kv` scope `admin`, atau env `ADMIN_API_KEY`? | Bentuk middleware |
| 2 | SPA pakai framework apa? | Fase 3; sarankan yang bisa build statis tanpa server |
| 3 | Apakah UI harus bisa mengedit `data`/`models` sebagai raw JSON? | Risiko vs fleksibilitas |
| 4 | Perlukah audit log perubahan konfigurasi? | Belum ada tabel; harus lewat `kv` |
