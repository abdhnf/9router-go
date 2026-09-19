import { useState, useEffect } from 'react'
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
  RefreshCw,
  Search,
  ExternalLink,
  ChevronRight
} from 'lucide-react'

// Types
interface SSEMetrics {
  activeRequests: number
  recentRequests: number
  errorProvider: string
  pending: {
    total: number
    byAccount?: Record<string, Record<string, number>>
  }
}

interface Connection {
  id: number
  provider: string
  name: string
  baseUrl: string
  isActive: boolean
  priority: number
  weight: number
  isLocked?: boolean
}

export default function App() {
  const [activeTab, setActiveTab] = useState<'overview' | 'connections' | 'combos' | 'keys' | 'logs' | 'settings'>('overview')
  const [apiKey, setApiKey] = useState<string>(() => localStorage.getItem('9router_admin_key') || '')
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(() => !!localStorage.getItem('9router_admin_key'))
  const [authError, setAuthError] = useState<string>('')
  
  // Realtime SSE State
  const [metrics, setMetrics] = useState<SSEMetrics>({
    activeRequests: 0,
    recentRequests: 0,
    errorProvider: '',
    pending: { total: 0 }
  })
  const [sseConnected, setSseConnected] = useState(false)

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

  // SSE Stream Listener
  useEffect(() => {
    if (!isAuthenticated) return

    const eventSource = new EventSource('/usage/stream')
    
    eventSource.onopen = () => {
      setSseConnected(true)
    }

    eventSource.onmessage = (event) => {
      try {
        const data: SSEMetrics = JSON.parse(event.data)
        setMetrics(data)
      } catch {
        // ignore parse error
      }
    }

    eventSource.onerror = () => {
      setSseConnected(false)
    }

    return () => {
      eventSource.close()
    }
  }, [isAuthenticated])

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
            active={activeTab === 'overview'} // reuse tab temporarily
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

        {/* Footer info & Logout */}
        <div className="p-4 border-t border-slate-800 mt-auto bg-slate-900/50">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs text-slate-400 font-mono">Role: Admin</span>
            <button
              onClick={handleLogout}
              className="text-xs text-rose-400 hover:text-rose-300 font-medium transition-colors"
            >
              Sign Out
            </button>
          </div>
          <p className="text-[11px] text-slate-500 truncate font-mono">
            {apiKey.slice(0, 10)}...{apiKey.slice(-4)}
          </p>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 flex flex-col min-w-0 bg-slate-950 overflow-y-auto">
        {/* Topbar */}
        <header className="h-14 border-b border-slate-800 px-6 flex items-center justify-between bg-slate-900/40 backdrop-blur shrink-0">
          <div className="flex items-center gap-2 text-sm">
            <span className="text-slate-400">Console</span>
            <ChevronRight className="w-3.5 h-3.5 text-slate-600" />
            <span className="text-slate-200 capitalize font-medium">{activeTab}</span>
          </div>

          <div className="flex items-center gap-4 text-xs font-mono">
            <div className="flex items-center gap-2 px-2.5 py-1 bg-slate-800/60 border border-slate-700/60 rounded-md">
              <span className="text-slate-400">Active RPS:</span>
              <span className="text-emerald-400 font-semibold">{metrics.activeRequests}</span>
            </div>
            <div className="flex items-center gap-2 px-2.5 py-1 bg-slate-800/60 border border-slate-700/60 rounded-md">
              <span className="text-slate-400">Queue:</span>
              <span className="text-cyan-400 font-semibold">{metrics.pending.total}</span>
            </div>
          </div>
        </header>

        {/* View Content */}
        <div className="p-6 max-w-7xl w-full mx-auto space-y-6">
          {activeTab === 'overview' && (
            <OverviewView metrics={metrics} sseConnected={sseConnected} onNavigate={setActiveTab} />
          )}

          {activeTab === 'connections' && (
            <ConnectionsView apiKey={apiKey} />
          )}

          {activeTab === 'combos' && (
            <CombosView apiKey={apiKey} />
          )}

          {activeTab === 'keys' && (
            <ApiKeysView apiKey={apiKey} />
          )}

          {activeTab === 'logs' && (
            <LogsView />
          )}

          {activeTab === 'settings' && (
            <SettingsView apiKey={apiKey} />
          )}
        </div>
      </main>
    </div>
  )
}

function NavItem({ icon, label, active, onClick, badge }: {
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
          : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 border border-transparent'
      }`}
    >
      <div className="flex items-center gap-2.5">
        {icon}
        <span>{label}</span>
      </div>
      {badge && (
        <span className="px-1.5 py-0.5 text-[10px] font-mono bg-emerald-500/20 text-emerald-300 rounded border border-emerald-500/30">
          {badge}
        </span>
      )}
    </button>
  )
}

// -------------------------------------------------------------
// Sub-Views
// -------------------------------------------------------------

function OverviewView({ metrics, sseConnected, onNavigate }: {
  metrics: SSEMetrics
  sseConnected: boolean
  onNavigate: (tab: any) => void
}) {
  return (
    <div className="space-y-6">
      {/* Telemetry Cards Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          label="Active Stream Requests"
          value={metrics.activeRequests.toString()}
          subtext="Concurrently processing"
          highlight={metrics.activeRequests > 0}
        />
        <MetricCard
          label="Recent Throughput"
          value={metrics.recentRequests.toString()}
          subtext="Requests in sample window"
        />
        <MetricCard
          label="Queue Backlog"
          value={metrics.pending.total.toString()}
          subtext="Pending connection slot"
          color="cyan"
        />
        <MetricCard
          label="Engine Status"
          value={sseConnected ? "HEALTHY" : "OFFLINE"}
          subtext={sseConnected ? "Zero CGO · 32K RPS Engine" : "Check daemon connection"}
          color={sseConnected ? "emerald" : "rose"}
        />
      </div>

      {/* Provider Health Alert if any */}
      {metrics.errorProvider && (
        <div className="p-4 bg-rose-500/10 border border-rose-500/20 rounded-xl flex items-start gap-3">
          <ShieldAlert className="w-5 h-5 text-rose-400 shrink-0 mt-0.5" />
          <div>
            <h4 className="text-sm font-semibold text-rose-300">Provider Backoff / Error Detected</h4>
            <p className="text-xs text-rose-400/90 mt-0.5">
              Provider <span className="font-mono font-bold text-white">{metrics.errorProvider}</span> is experiencing failure or cooldown.
            </p>
          </div>
        </div>
      )}

      {/* Quick Action Matrix */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-mono text-slate-400 uppercase tracking-wider">Providers</span>
              <Server className="w-4 h-4 text-emerald-400" />
            </div>
            <h3 className="text-base font-semibold text-white">Upstream Connections</h3>
            <p className="text-xs text-slate-400 mt-1">
              Kelola OpenAI, Anthropic, Gemini, Vertex, dan custom endpoints dengan merge lock.
            </p>
          </div>
          <button
            onClick={() => onNavigate('connections')}
            className="mt-4 w-full py-2 bg-slate-800 hover:bg-slate-700 text-xs font-medium rounded-lg text-slate-200 transition-colors"
          >
            Manage Connections →
          </button>
        </div>

        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-mono text-slate-400 uppercase tracking-wider">Combos</span>
              <Shuffle className="w-4 h-4 text-emerald-400" />
            </div>
            <h3 className="text-base font-semibold text-white">Model Routing & Fallback</h3>
            <p className="text-xs text-slate-400 mt-1">
              Atur strategi fusion, fallback, round-robin, dan sticky session otomatis.
            </p>
          </div>
          <button
            onClick={() => onNavigate('combos')}
            className="mt-4 w-full py-2 bg-slate-800 hover:bg-slate-700 text-xs font-medium rounded-lg text-slate-200 transition-colors"
          >
            Configure Combos →
          </button>
        </div>

        <div className="p-5 bg-slate-900 border border-slate-800 rounded-xl flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-mono text-slate-400 uppercase tracking-wider">Security</span>
              <Key className="w-4 h-4 text-emerald-400" />
            </div>
            <h3 className="text-base font-semibold text-white">API Keys & RBAC</h3>
            <p className="text-xs text-slate-400 mt-1">
              Buat client access tokens dan tetapkan otoritas admin via kv table.
            </p>
          </div>
          <button
            onClick={() => onNavigate('keys')}
            className="mt-4 w-full py-2 bg-slate-800 hover:bg-slate-700 text-xs font-medium rounded-lg text-slate-200 transition-colors"
          >
            Review Keys →
          </button>
        </div>
      </div>
    </div>
  )
}

function MetricCard({ label, value, subtext, highlight, color = 'emerald' }: {
  label: string
  value: string
  subtext: string
  highlight?: boolean
  color?: 'emerald' | 'cyan' | 'rose'
}) {
  const colorMap = {
    emerald: 'text-emerald-400',
    cyan: 'text-cyan-400',
    rose: 'text-rose-400'
  }

  return (
    <div className={`p-4 bg-slate-900 border rounded-xl transition-all ${
      highlight ? 'border-emerald-500/40 shadow-lg shadow-emerald-500/5' : 'border-slate-800'
    }`}>
      <span className="text-xs font-medium text-slate-400 block mb-1">{label}</span>
      <div className={`text-2xl font-bold font-mono ${colorMap[color]}`}>{value}</div>
      <span className="text-[11px] text-slate-500 mt-1 block">{subtext}</span>
    </div>
  )
}

function ConnectionsView({ apiKey }: { apiKey: string }) {
  const [connections, setConnections] = useState<Connection[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const fetchConnections = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await fetch('/api/admin/connections', {
        headers: { 'Authorization': `Bearer ${apiKey}` }
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await res.json()
      setConnections(data || [])
    } catch (err: any) {
      setError(err.message || 'Gagal mengambil data connections')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchConnections()
  }, [])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-white">Provider Connections</h2>
          <p className="text-xs text-slate-400">Upstream LLM endpoints yang dikelola oleh engine</p>
        </div>
        <button
          onClick={fetchConnections}
          className="p-2 bg-slate-900 hover:bg-slate-800 border border-slate-700 rounded-lg text-slate-300 transition-colors"
          title="Refresh Data"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {error && (
        <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-lg text-xs text-rose-400">
          {error}
        </div>
      )}

      {loading ? (
        <div className="space-y-2">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-16 bg-slate-900 border border-slate-800 rounded-lg animate-pulse" />
          ))}
        </div>
      ) : connections.length === 0 ? (
        <div className="p-12 text-center border border-dashed border-slate-800 rounded-xl bg-slate-900/20">
          <Server className="w-8 h-8 text-slate-600 mx-auto mb-3" />
          <h4 className="text-sm font-medium text-slate-300">Belum ada Connection</h4>
          <p className="text-xs text-slate-500 mt-1 max-w-sm mx-auto">
            Gunakan API atau form tambah koneksi untuk menghubungkan upstream API keys.
          </p>
        </div>
      ) : (
        <div className="border border-slate-800 rounded-xl overflow-hidden bg-slate-900 divide-y divide-slate-800">
          {connections.map(conn => (
            <div key={conn.id} className="p-4 flex items-center justify-between hover:bg-slate-800/40 transition-colors">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <span className="font-semibold text-sm text-white">{conn.name}</span>
                  <span className="px-2 py-0.5 text-[10px] font-mono bg-slate-800 text-slate-300 rounded border border-slate-700">
                    {conn.provider}
                  </span>
                  {conn.isActive ? (
                    <span className="px-1.5 py-0.5 text-[10px] font-mono bg-emerald-500/10 text-emerald-400 rounded border border-emerald-500/20">
                      ACTIVE
                    </span>
                  ) : (
                    <span className="px-1.5 py-0.5 text-[10px] font-mono bg-slate-800 text-slate-500 rounded">
                      DISABLED
                    </span>
                  )}
                </div>
                <div className="text-xs text-slate-400 font-mono">{conn.baseUrl}</div>
              </div>

              <div className="flex items-center gap-4 text-xs font-mono">
                <div className="text-right">
                  <div className="text-slate-400 text-[11px]">Weight / Priority</div>
                  <div className="text-slate-200">{conn.weight} / {conn.priority}</div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function CombosView({ apiKey }: { apiKey: string }) {
  return (
    <div className="p-8 text-center border border-dashed border-slate-800 rounded-xl bg-slate-900/30">
      <Shuffle className="w-8 h-8 text-slate-600 mx-auto mb-3" />
      <h3 className="text-sm font-semibold text-white">Combos & Strategy Engine</h3>
      <p className="text-xs text-slate-400 mt-1 max-w-md mx-auto">
        Jalur fallback & fusion model. Menyimpan strategi langsung di kv scope <code className="text-emerald-400 font-mono">comboStrategies</code>.
      </p>
    </div>
  )
}

function ApiKeysView({ apiKey }: { apiKey: string }) {
  return (
    <div className="p-8 text-center border border-dashed border-slate-800 rounded-xl bg-slate-900/30">
      <Key className="w-8 h-8 text-slate-600 mx-auto mb-3" />
      <h3 className="text-sm font-semibold text-white">API Keys Management</h3>
      <p className="text-xs text-slate-400 mt-1 max-w-md mx-auto">
        Kunci klien untuk menembak endpoint <code className="text-emerald-400 font-mono">/v1/chat/completions</code>.
      </p>
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

function SettingsView({ apiKey }: { apiKey: string }) {
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
