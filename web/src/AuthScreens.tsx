import { useState } from 'react'
import { Cpu, KeyRound, Lock, ShieldAlert, Eye, EyeOff } from 'lucide-react'
import { apiSend } from './api'

/** The built-in credential seeded on a fresh install. */
export const DEFAULT_PASSWORD = '123456'

const inputCls =
  'w-full px-3.5 py-2.5 bg-slate-950 border border-slate-800 rounded-lg text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-emerald-500/50 focus:border-emerald-500 font-mono transition-all'
const submitCls =
  'w-full py-2.5 px-4 bg-emerald-500 hover:bg-emerald-400 active:scale-[0.98] disabled:opacity-60 disabled:cursor-not-allowed text-slate-950 font-semibold rounded-lg text-sm transition-all shadow-lg shadow-emerald-500/10'

function PasswordField({
  label,
  value,
  onChange,
  placeholder,
  autoFocus,
  autoComplete,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  placeholder?: string
  autoFocus?: boolean
  autoComplete?: string
}) {
  const [reveal, setReveal] = useState(false)
  return (
    <div>
      <label className="block text-xs font-medium text-slate-300 mb-1.5 uppercase tracking-wider font-mono">
        {label}
      </label>
      <div className="relative">
        <input
          type={reveal ? 'text' : 'password'}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          autoFocus={autoFocus}
          autoComplete={autoComplete}
          className={`${inputCls} pr-10`}
        />
        <button
          type="button"
          onClick={() => setReveal((v) => !v)}
          aria-label={reveal ? 'Sembunyikan password' : 'Tampilkan password'}
          className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-300 transition-colors"
        >
          {reveal ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
        </button>
      </div>
    </div>
  )
}

/**
 * Login screen. The default credential is printed on the page on purpose: this
 * console ships with a known password so a fresh install is reachable, and the
 * operator is forced to change it immediately after (see ForcePasswordChange).
 */
export function LoginScreen({
  onSuccess,
  notice,
}: {
  onSuccess: (mustChangePassword: boolean) => void
  notice?: string
}) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!password.trim()) {
      setError('Password tidak boleh kosong.')
      return
    }
    setBusy(true)
    setError('')
    try {
      const res = await apiSend<{ mustChangePassword: boolean }>('/api/auth/login', 'POST', {
        password,
      })
      onSuccess(!!res.mustChangePassword)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login gagal.')
      setBusy(false)
    }
  }

  return (
    <div className="min-h-screen bg-slate-950 flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-xl p-6 shadow-2xl">
        <div className="flex items-center gap-3 mb-6">
          <div className="p-2.5 bg-emerald-500/10 border border-emerald-500/20 rounded-lg text-emerald-400">
            <Cpu className="w-6 h-6" />
          </div>
          <div>
            <h1 className="text-xl font-bold tracking-tight text-white">9Router Console</h1>
            <p className="text-xs text-slate-400 font-mono">High-Performance LLM Gateway</p>
          </div>
        </div>

        {notice && (
          <div className="mb-4 px-3 py-2 rounded-lg bg-amber-500/10 border border-amber-500/20">
            <p className="text-xs text-amber-300">{notice}</p>
          </div>
        )}

        <form onSubmit={submit} className="space-y-4">
          <PasswordField
            label="Password"
            value={password}
            onChange={setPassword}
            placeholder="••••••"
            autoFocus
            autoComplete="current-password"
          />

          {error && <p className="text-xs text-rose-400 font-medium">{error}</p>}

          <button type="submit" disabled={busy} className={submitCls}>
            {busy ? 'Memverifikasi…' : 'Masuk ke Console'}
          </button>
        </form>

        {/* Default credential, shown until the operator changes it. */}
        <div className="mt-6 pt-5 border-t border-slate-800/80">
          <div className="flex items-start gap-2.5 px-3 py-2.5 rounded-lg bg-slate-950/60 border border-slate-800">
            <KeyRound className="w-4 h-4 text-emerald-400 mt-0.5 shrink-0" />
            <div className="min-w-0">
              <p className="text-xs text-slate-300">
                Login default:{' '}
                <code className="font-mono text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded">
                  {DEFAULT_PASSWORD}
                </code>
              </p>
              <p className="text-[11px] text-slate-500 mt-1 leading-relaxed">
                Password ini wajib diganti saat login pertama. Ubah kapan saja di menu{' '}
                <span className="text-slate-400">Settings → Keamanan</span>.
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

/**
 * Blocking screen shown while the account still carries the built-in default
 * password. Nothing else in the console renders until the operator picks a real
 * password, so a LAN-reachable install cannot sit on `123456` indefinitely.
 */
export function ForcePasswordChange({ onDone }: { onDone: () => void }) {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (next.length < 8) {
      setError('Password baru minimal 8 karakter.')
      return
    }
    if (next !== confirm) {
      setError('Konfirmasi password tidak cocok.')
      return
    }
    setBusy(true)
    setError('')
    try {
      await apiSend('/api/auth/password', 'POST', { currentPassword: current, newPassword: next })
      onDone()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Gagal mengubah password.')
      setBusy(false)
    }
  }

  return (
    <div className="min-h-screen bg-slate-950 flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-xl p-6 shadow-2xl">
        <div className="flex items-center gap-3 mb-5">
          <div className="p-2.5 bg-amber-500/10 border border-amber-500/20 rounded-lg text-amber-400">
            <ShieldAlert className="w-6 h-6" />
          </div>
          <div>
            <h1 className="text-lg font-bold tracking-tight text-white">Ganti Password</h1>
            <p className="text-xs text-slate-400 font-mono">Wajib sebelum melanjutkan</p>
          </div>
        </div>

        <p className="text-xs text-slate-400 mb-5 leading-relaxed">
          Console ini masih memakai password default{' '}
          <code className="font-mono text-amber-400">{DEFAULT_PASSWORD}</code>. Pilih password baru
          untuk membuka dashboard.
        </p>

        <form onSubmit={submit} className="space-y-4">
          <PasswordField
            label="Password Saat Ini"
            value={current}
            onChange={setCurrent}
            placeholder={DEFAULT_PASSWORD}
            autoFocus
            autoComplete="current-password"
          />
          <PasswordField
            label="Password Baru"
            value={next}
            onChange={setNext}
            placeholder="min. 8 karakter"
            autoComplete="new-password"
          />
          <PasswordField
            label="Ulangi Password Baru"
            value={confirm}
            onChange={setConfirm}
            autoComplete="new-password"
          />

          {error && <p className="text-xs text-rose-400 font-medium">{error}</p>}

          <button type="submit" disabled={busy} className={submitCls}>
            {busy ? 'Menyimpan…' : 'Simpan & Lanjutkan'}
          </button>
        </form>
      </div>
    </div>
  )
}

/** Section inside Settings for changing the password at any time. */
export function PasswordSettings() {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [done, setDone] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setDone('')
    if (next.length < 8) {
      setError('Password baru minimal 8 karakter.')
      return
    }
    if (next !== confirm) {
      setError('Konfirmasi password tidak cocok.')
      return
    }
    setBusy(true)
    setError('')
    try {
      await apiSend('/api/auth/password', 'POST', { currentPassword: current, newPassword: next })
      setDone('Password berhasil diubah.')
      setCurrent('')
      setNext('')
      setConfirm('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Gagal mengubah password.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-6">
      <div className="flex items-center gap-3 mb-5">
        <div className="p-2 bg-emerald-500/10 border border-emerald-500/20 rounded-lg text-emerald-400">
          <Lock className="w-4 h-4" />
        </div>
        <div>
          <h3 className="text-sm font-semibold text-white">Keamanan Dashboard</h3>
          <p className="text-xs text-slate-400">Ubah password login console</p>
        </div>
      </div>

      <form onSubmit={submit} className="space-y-4 max-w-md">
        <PasswordField
          label="Password Saat Ini"
          value={current}
          onChange={setCurrent}
          autoComplete="current-password"
        />
        <PasswordField
          label="Password Baru"
          value={next}
          onChange={setNext}
          placeholder="min. 8 karakter"
          autoComplete="new-password"
        />
        <PasswordField
          label="Ulangi Password Baru"
          value={confirm}
          onChange={setConfirm}
          autoComplete="new-password"
        />

        {error && <p className="text-xs text-rose-400 font-medium">{error}</p>}
        {done && <p className="text-xs text-emerald-400 font-medium">{done}</p>}

        <button type="submit" disabled={busy} className={`${submitCls} w-auto px-6`}>
          {busy ? 'Menyimpan…' : 'Ubah Password'}
        </button>
      </form>
    </div>
  )
}
