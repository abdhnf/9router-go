# DASHBOARD-API — Kontrak Endpoint

> Referensi teknis untuk menghubungkan UI ke engine. Semua endpoint di sini **sudah berjalan**
> (kecuali yang ditandai ⚠️).
>
> Dasar: `DASHBOARD.md` §3–§4. Diverifikasi dengan `go test ./internal/handlers/admin/`.

---

## 1. Autentikasi

Semua endpoint butuh API key, dikirim dengan salah satu cara:

```
Authorization: Bearer <key>
X-API-Key: <key>
?key=<key>
```

Endpoint `/api/admin/*` butuh **hak admin**. Bila daftar admin belum pernah diisi, key pertama
yang berhasil masuk otomatis dipromosikan dan dicatat.

**403** berarti key valid tapi bukan admin:
```json
{"error":{"message":"API key ini tidak memiliki hak admin.","type":"permission_error","code":"insufficient_quota"}}
```

---

## 2. Bentuk Respons

Berhasil: `200` atau `201`, body JSON.

Gagal:
```json
{"error":{"message":"...","type":"invalid_request_error","code":"bad_request"}}
```

Kode yang mungkin: `400` body tidak valid · `401` tanpa key · `403` bukan admin ·
`404` tidak ditemukan · `405` method salah · `500` kesalahan server.

---

## 3. Meta — dipakai UI untuk mengisi pilihan

```
GET /api/admin/meta
```

```json
{
  "comboStrategies": ["sticky","round-robin","fallback","capacity","fusion"],
  "kvScopes": ["modelAliases","customModels","admin"],
  "cavemanLevels": ["lite","full","ultra","wenyan-ultra"],
  "ponytailLevels": ["lite","full","ultra"],
  "authTypes": ["api-key","oauth","cookie"],
  "notes": {
    "comboStrategy": "Disimpan di dalam JSON models, bukan kolom terpisah.",
    "connectionData": "Field data di-merge, bukan diganti. Berisi modelLock_* dan backoffLevel."
  }
}
```

> Jangan menuliskan daftar strategi di kode UI. Ambil dari sini agar tidak pernah berbeda
> dari engine.

---

## 4. Koneksi Provider

### Daftar
```
GET /api/admin/connections?provider=&activeOnly=
```
```json
{"connections":[
  {"id":"conn_a1b2","provider":"antigravity","authType":"oauth","name":"Utama",
   "email":"budi@example.com","priority":1,"isActive":1,
   "data":"{\"apiKey\":\"sk-...\",\"modelLock_gemini-3.8-flash\":1700000000000,\"backoffLevel\":3}",
   "createdAt":"2026-09-01T10:00:00Z","updatedAt":"2026-09-19T04:00:00Z"}
]}
```
Urutan: `priority` menaik, nilai kosong dianggap paling akhir. Lalu `updatedAt` menurun.

### Detail
```
GET /api/admin/connections/{id}
```

### Buat
```
POST /api/admin/connections
```
```json
{"provider":"openai","authType":"api-key","name":"CS","priority":2,
 "data":{"apiKey":"sk-..."}}
```
`provider` dan `authType` wajib. `id` dibuat otomatis bila tidak dikirim.

### Ubah — **PATCH, bukan PUT**
```
PATCH /api/admin/connections/{id}
```
```json
{"name":"CS Utama","priority":3,"isActive":true,
 "data":{"region":"id-jkt","staleKey":null}}
```

> **Field `data` di-MERGE, bukan diganti.**
> Isinya menyimpan status berjalan: `modelLock_<model>`, `backoffLevel`, dan kunci kuota
> antigravity. Menggantinya utuh akan menghapus semua kunci dan backoff yang sedang aktif —
> gejalanya muncul belakangan sebagai banjir 429 **tanpa pesan galat dan tanpa baris log**.
>
> Aturan untuk UI: kirim **hanya field yang berubah**. Jangan pernah mengirim ulang seluruh blob.
> Nilai `null` menghapus satu field.

### Hapus
```
DELETE /api/admin/connections/{id}
```

### Reset kesehatan
```
POST /api/admin/connections/{id}/health/reset
```
Membersihkan kunci model dan status backoff koneksi ini.

---

## 5. API Keys

```
GET    /api/admin/api-keys
POST   /api/admin/api-keys
PATCH  /api/admin/api-keys/{id}
DELETE /api/admin/api-keys/{id}
```

```json
{"apiKeys":[
  {"id":"key_x1","key":"sk-9f2a...","name":"Laptop Budi",
   "machineId":"macbook-pro","isActive":1,"createdAt":"2026-09-01T10:00:00Z"}
]}
```

Buat — bila `key` kosong, server membuat `sk-<hex>`:
```json
{"name":"Laptop Budi","machineId":"macbook-pro"}
```

Ubah — parsial:
```json
{"name":"Laptop Baru","isActive":false}
```

> `GET` mengembalikan kunci **apa adanya** karena operator perlu menyalinnya ke klien.
> Di UI, samarkan secara default dan sediakan tombol salin.

---

## 6. Combos

```
GET    /api/admin/combos
POST   /api/admin/combos
GET    /api/admin/combos/{id}
PATCH  /api/admin/combos/{id}
DELETE /api/admin/combos/{id}
```

Buat:
```json
{"name":"combo-wombo","strategy":"fusion",
 "models":["cc/claude-sonnet-4-6","ag/gemini-3.8-flash-high","cx/gpt-5.6"]}
```

`strategy` harus salah satu dari: `sticky`, `round-robin`, `fallback`, `capacity`, `fusion`.
Nilai lain ditolak `400`.

> **`strategy` tidak punya kolom di tabel `combos`.**
> Skema kanoniknya hanya `(id, name, kind, models, createdAt, updatedAt)`. Strategy disimpan di
> tabel `kv`, scope `comboStrategies`, dengan nama combo sebagai key.
>
> Ini bukan detail internal yang bisa diabaikan: sebelumnya `GetComboByName` selalu
> mengembalikan `"fallback"` karena memeriksa kolom yang tidak ada, sehingga **semua combo
> berjalan sebagai fallback** apa pun yang disimpan — tanpa galat. Sudah diperbaiki dan dijaga
> oleh test `TestComboStrategyRoundTripsThroughRouterPath`.
>
> Untuk UI: cukup kirim `strategy` seperti contoh di atas. Jangan mencoba menulisnya ke `models`.

---

## 7. Settings

```
GET /api/admin/settings
PUT /api/admin/settings
```

```json
{"settings":{
  "rtkEnabled": true,
  "cavemanEnabled": false,
  "cavemanLevel": "full",
  "ponytailEnabled": false,
  "ponytailLevel": "full",
  "headroomUrl": "http://localhost:8787",
  "headroomCodeAware": false,
  "headroomKompress": true,
  "headroomTimeoutMs": 3000,
  "autoUpdate": false,
  "providerStrategies": {
    "openrouter": {"proxyPoolId":"pool_x","rotateStrategy":"round-robin","stickyLimit":3}
  }
}}
```

`PUT` bersifat **merge** — kirim hanya field yang berubah:
```json
{"cavemanEnabled": true, "cavemanLevel": "ultra"}
```

`rotateStrategy` untuk strategi provider: `none`, `round-robin`, `random`, `sticky`.

---

## 8. KV — alias model & model kustom

```
GET    /api/admin/kv/{scope}/{key}
PUT    /api/admin/kv/{scope}/{key}
DELETE /api/admin/kv/{scope}/{key}
```

```json
{"scope":"modelAliases","key":"ag","value":"antigravity"}
```

Scope yang dibaca engine: `modelAliases`, `customModels`, `admin`.

> Scope di luar daftar itu tetap diterima, tetapi respons menyertakan peringatan karena
> penulisan tersebut tidak akan berpengaruh apa pun:
> ```json
> {"status":"ok","warning":"scope ini tidak dibaca engine; penulisan tidak akan berpengaruh."}
> ```
> Tampilkan peringatan ini di UI — ini menangkap salah ketik yang biasanya tidak terlihat.

---

## 9. Provider Nodes — provider kustom

```
GET    /api/admin/provider-nodes
POST   /api/admin/provider-nodes
PATCH  /api/admin/provider-nodes/{id}
DELETE /api/admin/provider-nodes/{id}
```

```json
{"provider":"openai-compatible","name":"Internal LLM",
 "data":{"baseUrl":"https://llm.internal/v1","apiKey":"sk-..."}}
```

Field `data` juga **merge** pada `PATCH`.

---

## 10. Statistik & pemantauan langsung

### Statistik
```
GET /api/usage/stats
GET /api/usage/stats?range=24h
```

### Aliran langsung (SSE)
```
GET /api/usage/stream
```

Bentuk muatan — **pakai apa adanya, jangan buat bentuk sendiri**:

```json
{
  "activeRequests": [
    {"model":"claude-sonnet-4-6","provider":"claude","account":"budi@example.com","count":2}
  ],
  "recentRequests": [
    {"timestamp":"2026-09-19T04:10:00Z","model":"gpt-5.6","provider":"codex",
     "promptTokens":1200,"completionTokens":340,"cachedTokens":800,"status":"success"}
  ],
  "errorProvider": "",
  "pending": {
    "byModel": {"claude-sonnet-4-6": 2},
    "byAccount": {"claude": {"budi@example.com": 2}}
  }
}
```

`cachedTokens` hanya muncul bila ada nilainya. `pending.byAccount` adalah map bersarang
(`provider → akun → jumlah`).

> Jangan memakai `EventSource` polos. Server mengirim keepalive setiap 25 detik; klien harus
> menyambung ulang dengan backoff dan **menampilkan status koneksi** ke operator. Diamnya
> aliran ini harus terlihat, bukan tersembunyi.

### Log konsol
```
GET    /translator/console-logs
GET    /translator/console-logs/stream
DELETE /translator/console-logs
```
Keepalive 25 detik. Event: `init`, `line`, `clear`.

### Trace
```
GET /debug/traces
```
Latensi p50/p95 per provider+model.

### Tool CLI
```
GET /cli-tools/all-statuses
```

### Katalog model
```
GET /v1/models
GET /v1/models/{kind}      # image | tts | web
GET /v1/models/*           # contoh: cc/claude-sonnet-4-6
GET /v1/models/info        # context_length, max_completion_tokens, ...
```

### Versi & pembaruan
```
GET  /api/version/status
GET  /api/version/check
POST /api/version/update
POST /api/version/auto-update
```

### Kesehatan
```
GET  /health                          # publik, tanpa key
POST /admin/health/reset?provider=&model=   # butuh key (bukan admin)
```

### Headroom
```
GET  /headroom/status
POST /headroom/start | /headroom/stop | /headroom/restart
GET  /headroom/extras    POST /headroom/extras    DELETE /headroom/extras
```

### Proxy pool — deploy relay
```
POST /proxy-pools/vercel-deploy
POST /proxy-pools/deno-deploy
POST /proxy-pools/cloudflare-deploy
```

### Impor OAuth
```
POST /api/oauth/{provider}/import
POST /api/oauth/codex/bulk-import
POST /api/oauth/grok-cli/bulk-import
GET  /api/oauth/kiro/social-authorize
POST /api/oauth/kiro/social-exchange
```

---

## 11. Catatan untuk Integrator UI

1. **Jangan menulis langsung ke SQLite.** Engine adalah satu-satunya penulis. Dua proses yang
   menulis ke file yang sama merusak state kunci dan backoff.
2. **Ambil daftar pilihan dari `/api/admin/meta`.** Jangan hardcode.
3. **`data` pada koneksi dan provider node selalu merge.** Kirim hanya field yang berubah.
4. **Perlakukan `models` pada combo sebagai JSON bebas.** `strategy` dikirim sebagai field
   terpisah, bukan di dalam `models`.
5. **Dua aliran SSE hidup bersamaan.** Tangani putusnya koneksi sebagai keadaan yang terlihat.
6. **`/health` tanpa key, sisanya perlu key.** `/api/admin/*` perlu hak admin.
