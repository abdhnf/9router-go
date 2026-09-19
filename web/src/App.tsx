import React, { useState, useEffect } from 'react'
import {
  Activity,
  Server,
  Shuffle,
  Key,
  BarChart3,
  Terminal,
  Settings,
  ShieldAlert,
  Cpu,
  Search,
  ChevronRight,
  Plus,
  Edit2,
  Trash2,
  CheckCircle2,
  AlertCircle,
  X,
  Lock
} from 'lucide-react'
import { useUsageStream, type UsageMetrics } from './useUsageStream'

interface Connection {
  id: string
  provider: string
  authType: string
  name: string
  email?: string
  priority: number
  isActive: number
  data?: string
  createdAt?: string
  updatedAt?: string
}

interface Combo {
  id: string
  name: string
  kind?: string
  models: string | string[]
  strategy?: string
  context_length?: number
}

interface ProviderOption {
  id: string
  noAuth: boolean
}

/**
 * Provider list comes from the engine (`/api/admin/meta`) rather than a
 * hardcoded array. A local list silently drifts: any provider the engine
 * supports but the array omits becomes unreachable from the console, which is
 * exactly how the free providers were missing.
 */
function useProviders(apiKey: string) {
  const [providers, setProviders] = useState<ProviderOption[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    fetch('/api/admin/meta', { headers: { Authorization: `Bearer ${apiKey}` } })
      .then((res) => (res.ok ? res.json() : Promise.reject(new Error(`HTTP ${res.status}`))))
      .then((meta) => {
        if (cancelled) return
        setProviders(Array.isArray(meta.providers) ? meta.providers : [])
      })
      .catch(() => {
        if (!cancelled) setProviders([])
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [apiKey])

  return { providers, loading }
}

const STRATEGY_OPTIONS = [
  { id: 'fallback', label: 'Fallback (Prioritas Berurutan)', desc: 'Coba model pertama, alihkan ke model berikutnya bila limit/error' },
  { id: 'round-robin', label: 'Round Robin (Beban Merata)', desc: 'Distribusikan request bergiliran ke seluruh model' },
  { id: 'sticky', label: 'Sticky Session', desc: 'Pertahankan model yang sama per conversation/session' },
  { id: 'capacity', label: 'Capacity Aware', desc: 'Pilih model dengan kuota dan ketersediaan terbaik' },
  { id: 'fusion', label: 'Fusion Multi-Model', desc: 'Jalankan paralel untuk benchmarking dan konsistensi' },
]

export default function App() {
  const [activeTab, setActiveTab] = useState<'overview' | 'connections' | 'combos' | 'keys' | 'logs' | 'settings'>('overview')
  const [apiKey, setApiKey] = useState<string>(() => localStorage.getItem('9router_admin_key') || '')
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(() => !!localStorage.getItem('9router_admin_key'))
  const [authError, setAuthError] = useState<string>('')

  // Realtime telemetry over authenticated SSE (fetch + ReadableStream).
  const { metrics, connected: sseConnected, error: streamError } = useUsageStream(apiKey, isAuthenticated)

  // A stored key that the engine rejects (rotated, revoked, or from another
  // instance) must send the operator back to the login gate instead of leaving
  // them on a dashboard where every panel silently stays empty.
  useEffect(() => {
    if (streamError === 'unauthorized') {
      localStorage.removeItem('9router_admin_key')
      setIsAuthenticated(false)
      setAuthError('API Key ditolak engine (401). Masukkan ulang key yang valid.')
    }
  }, [streamError])

  // Save admin key
  const handleLogin = (e: React.FormEvent) => {
    e.preventDefault()
    if (!apiKey.trim()) {
      setAuthError('API Key tidak boleh kosong')
      return
    }
    localStorage.setItem('9router_admin_key', apiKey.trim())
    setIsAuthenticated(true)
    setAuthError('')
  }

  const handleLogout = () => {
    localStorage.removeItem('9router_admin_key')
    setIsAuthenticated(false)
  }

  if (!isAuthenticated) {
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

          <form onSubmit={handleLogin} className="space-y-4">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5 uppercase tracking-wider font-mono">
                Admin API Key
              </label>
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="sk-..."
                className="w-full px-3.5 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-emerald-500/50 focus:border-emerald-500 font-mono transition-all"
                autoFocus
              />
              {authError && (
                <p className="text-xs text-rose-400 mt-1.5 font-medium">{authError}</p>
              )}
            </div>

            <button
              type="submit"
              className="w-full py-2.5 px-4 bg-emerald-500 hover:bg-emerald-400 active:scale-[0.98] text-slate-950 font-semibold rounded-lg text-sm transition-all shadow-lg shadow-emerald-500/10"
            >
              Sign In to Console
            </button>
          </form>

          <div className="mt-6 pt-6 border-t border-slate-800/80 text-center">
            <p className="text-xs text-slate-500">
              Direct connection to local engine daemon
            </p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col md:flex-row">
      {/* Sidebar */}
      <aside className="w-full md:w-64 bg-slate-900 border-r border-slate-800 flex flex-col shrink-0">
        {/* Brand */}
        <div className="p-4 border-b border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="p-1.5 bg-emerald-500/10 border border-emerald-500/20 rounded-md text-emerald-400">
              <Cpu className="w-5 h-5" />
            </div>
            <div>
              <span className="font-bold text-sm tracking-tight block">9Router Console</span>
              <span className="text-[10px] text-emerald-400 font-mono flex items-center gap-1">
                <span className={`w-1.5 h-1.5 rounded-full ${sseConnected ? 'bg-emerald-400 animate-pulse' : 'bg-amber-400'}`}></span>
                {sseConnected ? 'ENGINE LIVE' : 'CONNECTING'}
              </span>
            </div>
          </div>
        </div>

        {/* Navigation items */}
        <nav className="p-3 space-y-1 flex-1">
          <NavItem
            icon={<Activity className="w-4 h-4" />}
            label="Overview & Topologi"
            active={activeTab === 'overview'}
            onClick={() => setActiveTab('overview')}
            badge={metrics.activeRequests > 0 ? `${metrics.activeRequests} req` : undefined}
          />
          <NavItem
            icon={<Server className="w-4 h-4" />}
            label="Provider Connections"
            active={activeTab === 'connections'}
            onClick={() => setActiveTab('connections')}
          />
          <NavItem
            icon={<Shuffle className="w-4 h-4" />}
            label="Combos & Routing"
            active={activeTab === 'combos'}
            onClick={() => setActiveTab('combos')}
          />
          <NavItem
            icon={<Key className="w-4 h-4" />}
            label="API Keys & Access"
            active={activeTab === 'keys'}
            onClick={() => setActiveTab('keys')}
          />
          <NavItem
            icon={<BarChart3 className="w-4 h-4" />}
            label="Usage & Telemetry"
            active={false}
            onClick={() => setActiveTab('overview')}
          />
          <NavItem
            icon={<Terminal className="w-4 h-4" />}
            label="Translator Logs"
            active={activeTab === 'logs'}
            onClick={() => setActiveTab('logs')}
          />
          <NavItem
            icon={<Settings className="w-4 h-4" />}
            label="Settings"
            active={activeTab === 'settings'}
            onClick={() => setActiveTab('settings')}
          />
        </nav>

        {/* User Info & Sign Out footer (Clean & Consistent) */}
        <div className="p-3 border-t border-slate-800/80 bg-slate-900/50 mt-auto">
          <div className="flex items-center justify-between px-2 py-1.5 mb-2">
            <div>
              <p className="text-xs font-semibold text-slate-200">Admin Session</p>
              <p className="text-[10px] text-slate-400 font-mono">SQLite WAL Master</p>
            </div>
            <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-mono">
              RBAC OK
            </span>
          </div>
          <button
            onClick={handleLogout}
            className="w-full flex items-center justify-center gap-2 py-1.5 px-3 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 rounded-lg text-xs font-medium transition-colors"
          >
            Sign Out
          </button>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 flex flex-col min-w-0 bg-slate-950 overflow-y-auto">
        {/* Top Navbar */}
        <header className="h-14 border-b border-slate-800 px-6 flex items-center justify-between bg-slate-900/40 backdrop-blur shrink-0">
          <div className="flex items-center gap-2">
            <span className="text-xs font-medium text-slate-400">Console</span>
            <ChevronRight className="w-3.5 h-3.5 text-slate-600" />
            <span className="text-xs font-semibold text-slate-200 capitalize">
              {activeTab === 'overview' ? 'Overview & Topology' : activeTab}
            </span>
          </div>

          <div className="flex items-center gap-4 text-xs font-mono">
            <div className="flex items-center gap-2 px-2.5 py-1 bg-slate-900 border border-slate-800 rounded-md">
              <span className="text-slate-400">Active RPS:</span>
              <span className="text-emerald-400 font-bold tabular-nums">{metrics.recentRequests}</span>
            </div>
            <div className="flex items-center gap-2 px-2.5 py-1 bg-slate-900 border border-slate-800 rounded-md">
              <span className="text-slate-400">Queue:</span>
              <span className="text-slate-200 tabular-nums">{metrics.pending.total}</span>
            </div>
          </div>
        </header>

        {/* Tab Body */}
        <div className="p-6 max-w-7xl w-full mx-auto space-y-6">
          {activeTab === 'overview' && <OverviewView metrics={metrics} onNavigate={setActiveTab} />}
          {activeTab === 'connections' && <ConnectionsView apiKey={apiKey} />}
          {activeTab === 'combos' && <CombosView apiKey={apiKey} />}
          {activeTab === 'keys' && <ApiKeysView apiKey={apiKey} />}
          {activeTab === 'logs' && <LogsView />}
          {activeTab === 'settings' && <SettingsView />}
        </div>
      </main>
    </div>
  )
}

function NavItem({
  icon,
  label,
  active,
  onClick,
  badge
}: {
  icon: React.ReactNode
  label: string
  active: boolean
  onClick: () => void
  badge?: string
}) {
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs font-medium transition-all ${
        active
          ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
          : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
      }`}
    >
      <div className="flex items-center gap-2.5">
        {icon}
        <span>{label}</span>
      </div>
      {badge && (
        <span className="px-1.5 py-0.5 text-[10px] font-mono bg-emerald-500/20 text-emerald-400 rounded">
          {badge}
        </span>
      )}
    </button>
  )
}

function OverviewView({ metrics, onNavigate }: { metrics: UsageMetrics; onNavigate: (tab: any) => void }) {
  return (
    <div className="space-y-6">
      {/* 4 Metric Bento Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
          <div className="flex items-center justify-between text-slate-400 mb-2">
            <span className="text-xs font-medium">Active Stream Requests</span>
            <Activity className="w-4 h-4 text-emerald-400" />
          </div>
          <div className="text-2xl font-bold font-mono tabular-nums text-white">
            {metrics.activeRequests}
          </div>
          <p className="text-[11px] text-slate-500 mt-1">Concurrently processing</p>
        </div>

        <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
          <div className="flex items-center justify-between text-slate-400 mb-2">
            <span className="text-xs font-medium">Throughput Window</span>
            <BarChart3 className="w-4 h-4 text-emerald-400" />
          </div>
          <div className="text-2xl font-bold font-mono tabular-nums text-white">
            {metrics.recentRequests}
          </div>
          <p className="text-[11px] text-slate-500 mt-1">Requests in sample window</p>
        </div>

        <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
          <div className="flex items-center justify-between text-slate-400 mb-2">
            <span className="text-xs font-medium">Queue Backlog</span>
            <Shuffle className="w-4 h-4 text-amber-400" />
          </div>
          <div className="text-2xl font-bold font-mono tabular-nums text-white">
            {metrics.pending.total}
          </div>
          <p className="text-[11px] text-slate-500 mt-1">Pending connection slot</p>
        </div>

        <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
          <div className="flex items-center justify-between text-slate-400 mb-2">
            <span className="text-xs font-medium">Engine Lock State</span>
            <ShieldAlert className="w-4 h-4 text-emerald-400" />
          </div>
          <div className="text-base font-bold font-mono text-emerald-400 flex items-center gap-1.5 mt-1">
            <CheckCircle2 className="w-4 h-4" /> WAL MASTER
          </div>
          <p className="text-[11px] text-slate-500 mt-1">Single writer protected</p>
        </div>
      </div>

      {/* Quick Launch Actions */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 hover:border-slate-700 transition-colors">
          <div className="flex items-center gap-3 mb-3">
            <div className="p-2 bg-emerald-500/10 text-emerald-400 rounded-lg">
              <Server className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-white">Upstream Provider Connections</h3>
              <p className="text-xs text-slate-400">Kelola OpenAI, Anthropic, Gemini, Vertex, dan custom endpoints</p>
            </div>
          </div>
          <button
            onClick={() => onNavigate('connections')}
            className="w-full py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium rounded-lg transition-colors flex items-center justify-center gap-1.5"
          >
            Manage Connections <ChevronRight className="w-3.5 h-3.5" />
          </button>
        </div>

        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 hover:border-slate-700 transition-colors">
          <div className="flex items-center gap-3 mb-3">
            <div className="p-2 bg-emerald-500/10 text-emerald-400 rounded-lg">
              <Shuffle className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-white">Combos & Routing Matrix</h3>
              <p className="text-xs text-slate-400">Atur strategi fallback, round-robin, sticky, dan fusion multi-model</p>
            </div>
          </div>
          <button
            onClick={() => onNavigate('combos')}
            className="w-full py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium rounded-lg transition-colors flex items-center justify-center gap-1.5"
          >
            Configure Combos <ChevronRight className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  )
}

// ==========================================
// 1. PROVIDER CONNECTIONS VIEW & MODALS
// ==========================================
function ConnectionsView({ apiKey }: { apiKey: string }) {
  const [connections, setConnections] = useState<Connection[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [searchTerm, setSearchTerm] = useState('')

  const { providers } = useProviders(apiKey)

  // Modal State
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [editingConn, setEditingConn] = useState<Connection | null>(null)

  const fetchConnections = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/admin/connections', {
        headers: { Authorization: `Bearer ${apiKey}` }
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}: Gagal memuat data connections`)
      const data = await res.json()
      setConnections(data.connections || [])
      setError('')
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchConnections()
  }, [])

  const handleToggleActive = async (conn: Connection) => {
    try {
      const newActive = conn.isActive === 1 ? 0 : 1
      const res = await fetch(`/api/admin/connections/${conn.id}`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${apiKey}`
        },
        body: JSON.stringify({ isActive: newActive === 1 })
      })
      if (!res.ok) throw new Error('Gagal update status connection')
      setConnections(connections.map(c => c.id === conn.id ? { ...c, isActive: newActive } : c))
    } catch (err: any) {
      alert(err.message)
    }
  }

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Hapus provider connection "${name}"?`)) return
    try {
      const res = await fetch(`/api/admin/connections/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${apiKey}` }
      })
      if (!res.ok) throw new Error('Gagal menghapus connection')
      setConnections(connections.filter(c => c.id !== id))
    } catch (err: any) {
      alert(err.message)
    }
  }

  const filtered = connections.filter(c => 
    c.name?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    c.provider?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    c.email?.toLowerCase().includes(searchTerm.toLowerCase())
  )

  return (
    <div className="space-y-4">
      {/* Action Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 bg-slate-900 border border-slate-800 p-4 rounded-xl">
        <div>
          <h2 className="text-base font-semibold text-white">Upstream Provider Connections</h2>
          <p className="text-xs text-slate-400">Total {connections.length} endpoint terdaftar pada gateway</p>
        </div>

        <div className="flex items-center gap-2.5 w-full sm:w-auto">
          <div className="relative flex-1 sm:w-64">
            <Search className="w-3.5 h-3.5 absolute left-3 top-2.5 text-slate-500" />
            <input
              type="text"
              placeholder="Cari provider, nama, email..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-8 pr-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-emerald-500 font-mono"
            />
          </div>

          <button
            onClick={() => { setEditingConn(null); setIsModalOpen(true); }}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-emerald-500 hover:bg-emerald-400 active:scale-[0.98] text-slate-950 font-semibold rounded-lg text-xs transition-all shrink-0"
          >
            <Plus className="w-3.5 h-3.5" /> Tambah Provider
          </button>
        </div>
      </div>

      {error && (
        <div className="p-3 bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs rounded-lg flex items-center gap-2">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Table List */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
        {loading ? (
          <div className="p-8 text-center text-xs text-slate-500 font-mono animate-pulse">
            Memuat connections dari SQLite WAL database...
          </div>
        ) : filtered.length === 0 ? (
          <div className="p-8 text-center">
            <Server className="w-8 h-8 text-slate-600 mx-auto mb-2" />
            <p className="text-xs text-slate-400">Tidak ada provider connection yang cocok.</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-950/60 text-slate-400 border-b border-slate-800 font-mono uppercase tracking-wider">
                <tr>
                  <th className="py-2.5 px-4">Provider / Name</th>
                  <th className="py-2.5 px-4">Auth Type</th>
                  <th className="py-2.5 px-4 text-center">Priority</th>
                  <th className="py-2.5 px-4">Status & Lock State</th>
                  <th className="py-2.5 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/50">
                {filtered.map(conn => {
                  let parsedData: any = {}
                  try { if (conn.data) parsedData = JSON.parse(conn.data) } catch {}
                  const hasModelLocks = Object.keys(parsedData).some(k => k.startsWith('modelLock_') && parsedData[k])
                  const backoff = parsedData.backoffLevel || 0

                  return (
                    <tr key={conn.id} className="hover:bg-slate-850/40 transition-colors">
                      <td className="py-3 px-4">
                        <div className="font-medium text-slate-200">{conn.name || 'Unnamed'}</div>
                        <div className="text-[11px] text-slate-400 font-mono flex items-center gap-1.5 mt-0.5">
                          <span className="text-emerald-400">{conn.provider}</span>
                          {conn.email && <span>• {conn.email}</span>}
                        </div>
                      </td>
                      <td className="py-3 px-4 font-mono text-slate-300">
                        <span className="px-2 py-0.5 bg-slate-800 rounded border border-slate-700 text-[10px]">
                          {conn.authType}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-center font-mono font-bold tabular-nums text-slate-300">
                        {conn.priority}
                      </td>
                      <td className="py-3 px-4">
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => handleToggleActive(conn)}
                            className={`px-2 py-0.5 rounded text-[10px] font-mono font-medium transition-colors ${
                              conn.isActive === 1
                                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                                : 'bg-slate-800 text-slate-500 border border-slate-700'
                            }`}
                          >
                            {conn.isActive === 1 ? 'ACTIVE' : 'DISABLED'}
                          </button>
                          {hasModelLocks && (
                            <span className="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] bg-amber-500/10 text-amber-400 border border-amber-500/20 font-mono">
                              <Lock className="w-3 h-3" /> Lock
                            </span>
                          )}
                          {backoff > 0 && (
                            <span className="px-1.5 py-0.5 rounded text-[10px] bg-rose-500/10 text-rose-400 border border-rose-500/20 font-mono">
                              Backoff L{backoff}
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="py-3 px-4 text-right space-x-2">
                        <button
                          onClick={() => { setEditingConn(conn); setIsModalOpen(true); }}
                          className="p-1 hover:bg-slate-800 text-slate-400 hover:text-slate-200 rounded transition-colors"
                          title="Edit Connection"
                        >
                          <Edit2 className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => handleDelete(conn.id, conn.name)}
                          className="p-1 hover:bg-rose-500/20 text-slate-400 hover:text-rose-400 rounded transition-colors"
                          title="Delete Connection"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Modal Dialog Form (Merge Protected) */}
      {isModalOpen && (
        <ConnectionModal
          conn={editingConn}
          apiKey={apiKey}
          providers={providers}
          onClose={() => setIsModalOpen(false)}
          onSuccess={() => { setIsModalOpen(false); fetchConnections(); }}
        />
      )}
    </div>
  )
}

function ConnectionModal({
  conn,
  apiKey,
  providers,
  onClose,
  onSuccess
}: {
  conn: Connection | null
  apiKey: string
  providers: ProviderOption[]
  onClose: () => void
  onSuccess: () => void
}) {
  const isEdit = !!conn
  const [name, setName] = useState(conn?.name || '')
  const [provider, setProvider] = useState(conn?.provider || 'openai-compatible-chat')
  const [authType] = useState(conn?.authType || 'apikey')
  const [email, setEmail] = useState(conn?.email || '')
  const [priority, setPriority] = useState(conn?.priority ?? 1)
  const [isActive] = useState(conn?.isActive ?? 1)
  const [apiSecret, setApiSecret] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  // Prefill existing data if any
  useEffect(() => {
    if (conn?.data) {
      try {
        const d = JSON.parse(conn.data)
        if (d.apiKey) setApiSecret(d.apiKey)
        if (d.providerSpecificData?.baseUrl) setBaseUrl(d.providerSpecificData.baseUrl)
      } catch {}
    }
  }, [conn])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    setErr('')

    try {
      if (isEdit) {
        // PATCH with atomic JSON merge
        const patchBody: any = {
          name,
          email,
          priority: Number(priority),
          isActive: isActive === 1
        }
        // Kirim patch data bila ada perubahan apiKey / baseUrl
        const dataPatch: any = {}
        if (apiSecret) dataPatch.apiKey = apiSecret
        if (baseUrl) {
          dataPatch.providerSpecificData = { baseUrl }
        }
        if (Object.keys(dataPatch).length > 0) {
          patchBody.data = dataPatch
        }

        const res = await fetch(`/api/admin/connections/${conn.id}`, {
          method: 'PATCH',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${apiKey}`
          },
          body: JSON.stringify(patchBody)
        })
        if (!res.ok) {
          const b = await res.json()
          throw new Error(b.error?.message || 'Gagal menyimpan perubahan connection')
        }
      } else {
        // POST Create Full
        const createBody = {
          name,
          provider,
          authType,
          email,
          priority: Number(priority),
          isActive: isActive === 1,
          data: {
            apiKey: apiSecret,
            providerSpecificData: baseUrl ? { baseUrl } : {}
          }
        }
        const res = await fetch('/api/admin/connections', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${apiKey}`
          },
          body: JSON.stringify(createBody)
        })
        if (!res.ok) {
          const b = await res.json()
          throw new Error(b.error?.message || 'Gagal membuat provider connection')
        }
      }
      onSuccess()
    } catch (e: any) {
      setErr(e.message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-xl shadow-2xl overflow-hidden">
        <div className="p-4 border-b border-slate-800 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-white">
            {isEdit ? `Edit Provider Connection (${conn.name})` : 'Tambah Provider Connection'}
          </h3>
          <button onClick={onClose} className="text-slate-400 hover:text-white">
            <X className="w-4 h-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-5 space-y-4 text-xs">
          {err && (
            <div className="p-2.5 bg-rose-500/10 border border-rose-500/20 text-rose-400 rounded-lg">
              {err}
            </div>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-slate-400 mb-1 font-mono uppercase">Nama Label</label>
              <input
                type="text"
                required
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="misal: Gemini Primary"
                className="w-full px-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 focus:outline-none focus:border-emerald-500"
              />
            </div>
            <div>
              <label className="block text-slate-400 mb-1 font-mono uppercase">Provider Type</label>
              <select
                disabled={isEdit}
                value={provider}
                onChange={e => setProvider(e.target.value)}
                className="w-full px-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 focus:outline-none focus:border-emerald-500 disabled:opacity-50"
              >
                {providers.map(p => (
                  <option key={p.id} value={p.id}>
                    {p.noAuth ? `${p.id} — gratis (tanpa API key)` : p.id}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-slate-400 mb-1 font-mono uppercase">Email / Account ID</label>
              <input
                type="text"
                value={email}
                onChange={e => setEmail(e.target.value)}
                placeholder="opsional"
                className="w-full px-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 focus:outline-none focus:border-emerald-500 font-mono"
              />
            </div>
            <div>
              <label className="block text-slate-400 mb-1 font-mono uppercase">Priority (1 = Tertinggi)</label>
              <input
                type="number"
                min="1"
                max="99"
                value={priority}
                onChange={e => setPriority(Number(e.target.value))}
                className="w-full px-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 focus:outline-none focus:border-emerald-500 font-mono tabular-nums"
              />
            </div>
          </div>

          <div>
            <label className="block text-slate-400 mb-1 font-mono uppercase">
              API Key / Token Secret {isEdit && '(Kosongkan bila tidak ingin diubah)'}
            </label>
            <input
              type="password"
              value={apiSecret}
              onChange={e => setApiSecret(e.target.value)}
              placeholder="sk-... / token secret"
              className="w-full px-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 focus:outline-none focus:border-emerald-500 font-mono"
            />
          </div>

          {provider.includes('compatible') && (
            <div>
              <label className="block text-slate-400 mb-1 font-mono uppercase">Base URL Endpoint</label>
              <input
                type="url"
                value={baseUrl}
                onChange={e => setBaseUrl(e.target.value)}
                placeholder="https://api.groq.com/openai/v1"
                className="w-full px-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 focus:outline-none focus:border-emerald-500 font-mono"
              />
            </div>
          )}

          {isEdit && (
            <div className="p-2.5 bg-emerald-500/10 border border-emerald-500/20 rounded-lg text-emerald-400 text-[11px] font-mono">
              🛡️ <strong>Runtime Merge Active:</strong> Model lock status dan backoff level tersimpan di DB tidak akan tertimpa.
            </div>
          )}

          <div className="pt-2 flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg font-medium transition-colors"
            >
              Batal
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="px-4 py-1.5 bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-semibold rounded-lg transition-colors disabled:opacity-50"
            >
              {submitting ? 'Menyimpan...' : 'Simpan Connection'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ==========================================
// 2. COMBOS & ROUTING MATRIX VIEW & MODALS
// ==========================================
function CombosView({ apiKey }: { apiKey: string }) {
  const [combos, setCombos] = useState<Combo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [editingCombo, setEditingCombo] = useState<Combo | null>(null)

  const fetchCombos = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/admin/combos', {
        headers: { Authorization: `Bearer ${apiKey}` }
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}: Gagal memuat combos`)
      const data = await res.json()
      setCombos(data.combos || [])
      setError('')
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchCombos()
  }, [])

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Hapus combo model "${name}"?`)) return
    try {
      const res = await fetch(`/api/admin/combos/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${apiKey}` }
      })
      if (!res.ok) throw new Error('Gagal menghapus combo')
      setCombos(combos.filter(c => c.id !== id))
    } catch (err: any) {
      alert(err.message)
    }
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 bg-slate-900 border border-slate-800 p-4 rounded-xl">
        <div>
          <h2 className="text-base font-semibold text-white">Combos & Routing Rules</h2>
          <p className="text-xs text-slate-400">Grup virtual model dengan multi-tier fallback dan round-robin</p>
        </div>

        <button
          onClick={() => { setEditingCombo(null); setIsModalOpen(true); }}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-emerald-500 hover:bg-emerald-400 active:scale-[0.98] text-slate-950 font-semibold rounded-lg text-xs transition-all shrink-0"
        >
          <Plus className="w-3.5 h-3.5" /> Buat Combo Baru
        </button>
      </div>

      {error && (
        <div className="p-3 bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs rounded-lg flex items-center gap-2">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Grid of Combo Cards */}
      {loading ? (
        <div className="p-8 text-center text-xs text-slate-500 font-mono animate-pulse">
          Memuat konfigurasi combo routing...
        </div>
      ) : combos.length === 0 ? (
        <div className="p-8 text-center bg-slate-900 border border-slate-800 rounded-xl">
          <Shuffle className="w-8 h-8 text-slate-600 mx-auto mb-2" />
          <p className="text-xs text-slate-400">Belum ada Combo model yang dibuat.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {combos.map(combo => {
            let modelList: string[] = []
            try {
              if (Array.isArray(combo.models)) modelList = combo.models
              else if (typeof combo.models === 'string') modelList = JSON.parse(combo.models)
            } catch {}

            const strategy = combo.strategy || 'fallback'

            return (
              <div key={combo.id} className="bg-slate-900 border border-slate-800 rounded-xl p-4 flex flex-col justify-between space-y-4 hover:border-slate-700 transition-colors">
                <div>
                  <div className="flex items-start justify-between gap-2 mb-2">
                    <div>
                      <h3 className="text-sm font-bold text-white flex items-center gap-2">
                        {combo.name}
                        <span className="px-2 py-0.5 bg-slate-800 text-emerald-400 border border-slate-700 rounded text-[10px] font-mono capitalize">
                          {strategy}
                        </span>
                      </h3>
                      <p className="text-[11px] text-slate-400 font-mono mt-0.5">
                        ID: {combo.id}
                      </p>
                    </div>

                    <div className="flex items-center gap-1">
                      <button
                        onClick={() => { setEditingCombo(combo); setIsModalOpen(true); }}
                        className="p-1 hover:bg-slate-800 text-slate-400 hover:text-slate-200 rounded"
                        title="Edit Combo"
                      >
                        <Edit2 className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => handleDelete(combo.id, combo.name)}
                        className="p-1 hover:bg-rose-500/20 text-slate-400 hover:text-rose-400 rounded"
                        title="Delete Combo"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>

                  {/* Models Stack */}
                  <div className="space-y-1.5 mt-3">
                    <span className="text-[10px] text-slate-500 font-mono uppercase tracking-wider block">
                      Tier Models List ({modelList.length} targets)
                    </span>
                    <div className="space-y-1">
                      {modelList.map((m, idx) => (
                        <div key={idx} className="flex items-center gap-2 px-2.5 py-1.5 bg-slate-950 border border-slate-850 rounded-lg text-xs font-mono">
                          <span className="w-4 h-4 rounded bg-slate-800 text-slate-400 flex items-center justify-center text-[10px]">
                            {idx + 1}
                          </span>
                          <span className="text-slate-200 truncate flex-1">{m}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>

                <div className="pt-2 border-t border-slate-800/80 flex items-center justify-between text-[11px] text-slate-500 font-mono">
                  <span>Context: {combo.context_length ? `${combo.context_length} tokens` : 'Auto'}</span>
                  <span className="text-emerald-400">kv scope: comboStrategies</span>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Modal Dialog Form Combos */}
      {isModalOpen && (
        <ComboModal
          combo={editingCombo}
          apiKey={apiKey}
          onClose={() => setIsModalOpen(false)}
          onSuccess={() => { setIsModalOpen(false); fetchCombos(); }}
        />
      )}
    </div>
  )
}

function ComboModal({
  combo,
  apiKey,
  onClose,
  onSuccess
}: {
  combo: Combo | null
  apiKey: string
  onClose: () => void
  onSuccess: () => void
}) {
  const isEdit = !!combo
  const [name, setName] = useState(combo?.name || '')
  const [strategy, setStrategy] = useState(combo?.strategy || 'fallback')
  const [modelsText, setModelsText] = useState(() => {
    if (!combo?.models) return '[]'
    if (typeof combo.models === 'string') return combo.models
    return JSON.stringify(combo.models, null, 2)
  })
  const [submitting, setSubmitting] = useState(false)
  const [err, setErr] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    setErr('')

    try {
      let parsedModels
      try {
        parsedModels = JSON.parse(modelsText)
        if (!Array.isArray(parsedModels)) throw new Error('Models harus berupa JSON array string!')
      } catch (err: any) {
        throw new Error(`Format JSON Models tidak valid: ${err.message}`)
      }

      if (isEdit) {
        const res = await fetch(`/api/admin/combos/${combo.id}`, {
          method: 'PATCH',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${apiKey}`
          },
          body: JSON.stringify({
            name,
            strategy,
            models: parsedModels
          })
        })
        if (!res.ok) {
          const b = await res.json()
          throw new Error(b.error?.message || 'Gagal update combo')
        }
      } else {
        const res = await fetch('/api/admin/combos', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${apiKey}`
          },
          body: JSON.stringify({
            name,
            strategy,
            models: parsedModels
          })
        })
        if (!res.ok) {
          const b = await res.json()
          throw new Error(b.error?.message || 'Gagal membuat combo')
        }
      }
      onSuccess()
    } catch (e: any) {
      setErr(e.message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-xl shadow-2xl overflow-hidden">
        <div className="p-4 border-b border-slate-800 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-white">
            {isEdit ? `Edit Combo (${combo.name})` : 'Buat Combo Routing Baru'}
          </h3>
          <button onClick={onClose} className="text-slate-400 hover:text-white">
            <X className="w-4 h-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-5 space-y-4 text-xs">
          {err && (
            <div className="p-2.5 bg-rose-500/10 border border-rose-500/20 text-rose-400 rounded-lg">
              {err}
            </div>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-slate-400 mb-1 font-mono uppercase">Nama Combo</label>
              <input
                type="text"
                required
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="misal: The-Dev"
                className="w-full px-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 focus:outline-none focus:border-emerald-500 font-mono"
              />
            </div>
            <div>
              <label className="block text-slate-400 mb-1 font-mono uppercase">Routing Strategy</label>
              <select
                value={strategy}
                onChange={e => setStrategy(e.target.value)}
                className="w-full px-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 focus:outline-none focus:border-emerald-500"
              >
                {STRATEGY_OPTIONS.map(s => (
                  <option key={s.id} value={s.id}>{s.label}</option>
                ))}
              </select>
            </div>
          </div>

          <div>
            <label className="block text-slate-400 mb-1 font-mono uppercase">
              Target Models (JSON Array Format)
            </label>
            <textarea
              rows={6}
              value={modelsText}
              onChange={e => setModelsText(e.target.value)}
              placeholder='["openai/gpt-4o", "gemini/gemini-2.5-pro"]'
              className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 focus:outline-none focus:border-emerald-500 font-mono text-xs"
            />
            <p className="text-[11px] text-slate-500 mt-1">
              Urutan array menentukan prioritas tier saat fallback aktif.
            </p>
          </div>

          <div className="pt-2 flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg font-medium transition-colors"
            >
              Batal
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="px-4 py-1.5 bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-semibold rounded-lg transition-colors disabled:opacity-50"
            >
              {submitting ? 'Menyimpan...' : 'Simpan Combo'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function ApiKeysView({ apiKey }: { apiKey: string }) {
  const [keys, setKeys] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    fetch('/api/admin/api-keys', {
      headers: { Authorization: `Bearer ${apiKey}` }
    })
      .then(res => res.json())
      .then(d => { setKeys(d.apiKeys || []); setLoading(false); })
      .catch(e => { setError(e.message); setLoading(false); })
  }, [])

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
      <h2 className="text-sm font-semibold text-white mb-2">API Keys & RBAC Roles</h2>
      <p className="text-xs text-slate-400 mb-4">Daftar client token yang diizinkan memanggil endpoint engine.</p>
      {loading ? (
        <div className="text-xs text-slate-500 font-mono">Memuat keys...</div>
      ) : error ? (
        <div className="p-3 bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs rounded-lg flex items-center gap-2">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      ) : (
        <div className="space-y-2">
          {keys.map((k: any) => (
            <div key={k.id} className="p-3 bg-slate-950 border border-slate-800 rounded-lg flex items-center justify-between text-xs font-mono">
              <div>
                <span className="font-semibold text-white">{k.name || 'Unnamed'}</span>
                <span className="text-slate-500 ml-2">ID: {k.id}</span>
              </div>
              <span className="text-emerald-400">Active</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function LogsView() {
  return (
    <div className="p-8 text-center border border-dashed border-slate-800 rounded-xl bg-slate-900/30">
      <Terminal className="w-8 h-8 text-slate-600 mx-auto mb-3" />
      <h3 className="text-sm font-semibold text-white">Realtime Translator Logs</h3>
      <p className="text-xs text-slate-400 mt-1 max-w-md mx-auto">
        Stream live inspeksi request translation OpenAI ↔ Anthropic ↔ Gemini.
      </p>
    </div>
  )
}

function SettingsView() {
  return (
    <div className="p-8 text-center border border-dashed border-slate-800 rounded-xl bg-slate-900/30">
      <Settings className="w-8 h-8 text-slate-600 mx-auto mb-3" />
      <h3 className="text-sm font-semibold text-white">System Settings</h3>
      <p className="text-xs text-slate-400 mt-1 max-w-md mx-auto">
        Konfigurasi Token Saver, Auto Update, dan Headroom process throttle.
      </p>
    </div>
  )
}
