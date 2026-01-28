import { useState, useEffect, createContext, useContext, useCallback } from 'react'
import { HashRouter as Router, Routes, Route, NavLink, useNavigate, useLocation } from 'react-router-dom'
import { ModalProvider } from './components/Modal.jsx'
import {
  Image, BarChart3, FileText, RefreshCw, Save, Download, Upload,
  Clipboard, Globe, Trash2, Check, X, AlertTriangle, Search, Edit,
  Package, Folder, Truck, Inbox, Loader
} from 'lucide-react'
import StoreAddImage from './pages/StoreAddImage.jsx'
import StoreAddChart from './pages/StoreAddChart.jsx'
import StoreAddFile from './pages/StoreAddFile.jsx'
import StoreSync from './pages/StoreSync.jsx'
import StoreSave from './pages/StoreSave.jsx'
import StoreLoad from './pages/StoreLoad.jsx'
import StoreExtract from './pages/StoreExtract.jsx'
import StoreCopy from './pages/StoreCopy.jsx'
import StoreRemove from './pages/StoreRemove.jsx'
import Manifests from './pages/Manifests.jsx'
import StoreContents from './pages/StoreContents.jsx'
import Hauls from './pages/Hauls.jsx'
import Login from './pages/Login.jsx'
import './App.css'

// === Context for Auth ===
const AuthContext = createContext()

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return context
}

function AuthProvider({ children }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [authEnabled, setAuthEnabled] = useState(false)
  const [loading, setLoading] = useState(true)

  const checkAuth = useCallback(async () => {
    try {
      const res = await fetch('/api/auth/validate')
      if (res.ok) {
        const data = await res.json()
        setIsAuthenticated(data.authenticated)
        setAuthEnabled(data.authEnabled)
      }
    } catch (err) {
      console.error('Failed to check auth status:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    checkAuth()
  }, [checkAuth])

  const login = async (password) => {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password })
    })
    if (res.ok) {
      const data = await res.json()
      if (data.success) {
        setIsAuthenticated(true)
        return true
      }
    }
    return false
  }

  const logout = async () => {
    try {
      await fetch('/api/auth/logout', { method: 'POST' })
    } catch (err) {
      console.error('Logout error:', err)
    }
    setIsAuthenticated(false)
  }

  return (
    <AuthContext.Provider value={{ isAuthenticated, authEnabled, loading, login, logout, checkAuth }}>
      {children}
    </AuthContext.Provider>
  )
}

// === Context for Jobs ===
const JobsContext = createContext()

export function useJobs() {
  const context = useContext(JobsContext)
  if (!context) {
    throw new Error('useJobs must be used within JobsProvider')
  }
  return context
}

function JobsProvider({ children }) {
  const [jobs, setJobs] = useState([])
  const [runningJobCount, setRunningJobCount] = useState(0)

  // Normalize job data from API (handles both capitalized and lowercase fields)
  const normalizeJob = (data) => ({
    id: data.ID || data.id,
    command: data.Command || data.command,
    args: data.Args || data.args || [],
    status: (data.Status || data.status || '').toLowerCase(),
    exitCode: data.ExitCode ?? data.exitCode,
    startedAt: data.StartedAt || data.startedAt,
    completedAt: data.CompletedAt || data.completedAt,
    createdAt: data.CreatedAt || data.createdAt,
    result: data.Result?.String || data.result,
    envOverrides: data.EnvOverrides || data.envOverrides
  })

  const fetchJobs = useCallback(async () => {
    try {
      const res = await fetch('/api/jobs')
      if (res.ok) {
        const data = await res.json()
        const jobsData = Array.isArray(data) ? data : []
        setJobs(jobsData.map(normalizeJob))
        const running = jobsData.filter(j => {
          const status = (j.Status || j.status || '').toLowerCase()
          return status === 'running' || status === 'queued'
        }).length
        setRunningJobCount(running)
      }
    } catch (err) {
      console.error('Failed to fetch jobs:', err)
    }
  }, [])

  useEffect(() => {
    fetchJobs()
    const interval = setInterval(fetchJobs, 2000)
    return () => clearInterval(interval)
  }, [fetchJobs])

  const createJob = async (command, args = [], envOverrides = {}) => {
    const res = await fetch('/api/jobs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ command, args, envOverrides })
    })
    if (res.ok) {
      await fetchJobs()
      return await res.json()
    }
    throw new Error('Failed to create job')
  }

  const deleteAllJobs = async () => {
    const res = await fetch('/api/jobs', { method: 'DELETE' })
    if (res.ok) {
      setJobs([])
      setRunningJobCount(0)
      return true
    }
    throw new Error('Failed to delete jobs')
  }

  return (
    <JobsContext.Provider value={{ jobs, runningJobCount, fetchJobs, createJob, deleteAllJobs }}>
      {children}
    </JobsContext.Provider>
  )
}

// === Components ===

function StatusBadge({ status, className = '' }) {
  const badges = {
    queued: 'badge-info',
    running: 'badge-warning',
    succeeded: 'badge-success',
    failed: 'badge-error'
  }
  return <span className={`badge ${badges[status] || ''} ${className}`}>{status}</span>
}

function JobIndicator() {
  const { runningJobCount, fetchJobs } = useJobs()
  const navigate = useNavigate()

  useEffect(() => {
    fetchJobs()
  }, [fetchJobs])

  return (
    <button
      className={`job-indicator ${runningJobCount > 0 ? 'running' : ''}`}
      onClick={() => navigate('/jobs')}
    >
      <span className="status-dot"></span>
      <span>{runningJobCount} job{runningJobCount !== 1 ? 's' : ''} running</span>
    </button>
  )
}

// === Sidebar ===

function Sidebar() {
  const [isOpen, setIsOpen] = useState(false)

  const navGroups = [
    {
      title: 'Main',
      items: [
        { path: '/', label: 'Dashboard' },
        { path: '/store', label: 'Store' },
        { path: '/store/contents', label: 'Store Contents' },
        { path: '/hauls', label: 'Hauls' },
        { path: '/manifests', label: 'Manifests' }
      ]
    },
    {
      title: 'Operations',
      items: [
        { path: '/serve', label: 'Serve' },
        { path: '/registry', label: 'Registry Login' }
      ]
    },
    {
      title: 'System',
      items: [
        { path: '/jobs', label: 'Job History' },
        { path: '/settings', label: 'Settings' }
      ]
    }
  ]

  return (
    <>
      <aside className={`sidebar ${isOpen ? 'open' : ''}`}>
        <div className="sidebar-header">
          <img src="/hauler-logo.svg" alt="Hauler" className="sidebar-logo" />
        </div>
        <nav className="sidebar-nav">
          {navGroups.map((group, i) => (
            <div key={i} className="sidebar-section">
              <div className="sidebar-section-title">{group.title}</div>
              {group.items.map(item => (
                <NavLink
                  key={item.path}
                  to={item.path}
                  className="nav-link"
                  end
                  onClick={() => setIsOpen(false)}
                >
                  {item.label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <div className="sidebar-footer">
          <div className="sidebar-attribution">Hauler-UI built by <a href="https://alphabravo.io" target="_blank" rel="noopener noreferrer">AlphaBravo</a></div>
        </div>
      </aside>
    </>
  )
}

// === Pages ===

function Dashboard() {
  const [health, setHealth] = useState(null)
  const [capabilities, setCapabilities] = useState(null)

  useEffect(() => {
    fetch('/healthz')
      .then(res => res.json())
      .then(data => setHealth(data))
      .catch(() => setHealth({ status: 'error' }))

    fetch('/api/hauler/capabilities')
      .then(res => res.json())
      .then(data => setCapabilities(data))
  }, [])

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Dashboard</h1>
          <p className="page-subtitle">Overview of your hauler system</p>
        </div>
      </div>

      <div className="card">
        <div className="card-title">System Status</div>
        {health && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <StatusBadge status={health.status === 'ok' ? 'succeeded' : 'failed'} />
              <span style={{ color: 'var(--text-secondary)' }}>
                Backend: {health.status}
              </span>
            </div>
            {capabilities && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <span style={{ color: 'var(--text-secondary)' }}>Hauler CLI:</span>
                <code style={{ fontSize: '0.9rem', color: 'var(--accent-amber)' }}>
                  {capabilities.version.Full || 'Unknown'}
                </code>
              </div>
            )}
          </div>
        )}
      </div>

      <div className="card">
        <div className="card-title">Quick Actions</div>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          <NavLink to="/store" className="btn">View Store</NavLink>
          <NavLink to="/manifests" className="btn">Manage Manifests</NavLink>
          <NavLink to="/jobs" className="btn">Job History</NavLink>
        </div>
      </div>

      <div className="card">
        <div className="card-title">Getting Started</div>
        <p style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
          Welcome to Hauler UI. Use the navigation sidebar to manage your container store,
          create manifests, run hauls, and monitor background jobs.
        </p>
      </div>
    </div>
  )
}

function Store() {
  const [config, setConfig] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    // Fetch config
    fetch('/api/config')
      .then(res => res.json())
      .then(data => setConfig(data))
      .catch(err => setError('Failed to load config: ' + err.message))
  }, [])

  // Store operations mapped to routes
  const storeOperations = [
    { id: 'add-image', name: 'Add Image', description: 'Add container images to the store', icon: Image, route: '/store/add' },
    { id: 'add-chart', name: 'Add Chart', description: 'Add Helm charts to the store', icon: BarChart3, route: '/store/add-chart' },
    { id: 'add-file', name: 'Add File', description: 'Add local files or remote URLs to the store', icon: FileText, route: '/store/add-file' },
    { id: 'sync', name: 'Sync', description: 'Sync store from manifest files', icon: RefreshCw, route: '/store/sync' },
    { id: 'save', name: 'Save', description: 'Package store as a portable archive', icon: Save, route: '/store/save' },
    { id: 'load', name: 'Load', description: 'Load an archive into the store', icon: Download, route: '/store/load' },
    { id: 'extract', name: 'Extract', description: 'Extract artifacts from the store', icon: Upload, route: '/store/extract' },
    { id: 'copy', name: 'Copy', description: 'Copy store to registry or directory', icon: Clipboard, route: '/store/copy' },
    { id: 'serve', name: 'Serve', description: 'Serve registry or fileserver', icon: Globe, route: '/serve' },
    { id: 'remove', name: 'Remove', description: 'Remove artifacts from store (experimental)', icon: Trash2, route: '/store/remove' },
  ]

  // Related pages in the app
  const relatedPages = [
    { name: 'Store Contents', description: 'Browse contents of your content store', route: '/store/contents' },
    { name: 'Hauls', description: 'Manage haul archive files', route: '/hauls' },
    { name: 'Manifests', description: 'Create and manage hauler manifests', route: '/manifests' },
    { name: 'Registry Login', description: 'Manage container registry credentials', route: '/registry' },
  ]

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Store</h1>
          <p className="page-subtitle">Overview of your container store and available operations</p>
        </div>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {/* Store Paths */}
      {config && (
        <div className="card">
          <div className="card-title">Store Configuration</div>
          <table className="data-table">
            <tbody>
              <tr>
                <td style={{ width: '180px' }}>Store Directory</td>
                <td className="primary">
                  <code>{config.haulerStoreDir || '-'}</code>
                </td>
                <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  {config.haulerStoreEnv || 'HAULER_STORE_DIR'}
                </td>
              </tr>
              <tr>
                <td>Hauler Directory</td>
                <td className="primary">
                  <code>{config.haulerDir || '-'}</code>
                </td>
                <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  {config.haulerDirEnv || 'HAULER_DIR'}
                </td>
              </tr>
              <tr>
                <td>Temp Directory</td>
                <td className="primary">
                  <code>{config.haulerTempDir || '-'}</code>
                </td>
                <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  {config.haulerTempEnv || 'HAULER_TEMP_DIR'}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      )}

      {/* Store Operations */}
      <div className="card">
        <div className="card-title">Store Operations</div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: '0.75rem' }}>
          {storeOperations.map(op => (
            <NavLink
              key={op.id}
              to={op.route}
              className="operation-card"
              style={{ textDecoration: 'none' }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                <op.icon size={24} style={{ color: 'var(--accent-amber-dim)' }} />
                <div style={{ flex: 1 }}>
                  <div style={{ fontWeight: '500', color: 'var(--text-primary)' }}>{op.name}</div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{op.description}</div>
                </div>
              </div>
            </NavLink>
          ))}
        </div>
      </div>

      {/* Related Pages */}
      <div className="card">
        <div className="card-title">Related Pages</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          {relatedPages.map(page => (
            <NavLink
              key={page.name}
              to={page.route}
              className="nav-link"
              style={{ padding: '0.5rem 0.75rem' }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ fontWeight: '500' }}>{page.name}</span>
                <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginRight: '0.5rem' }}>
                  {page.description}
                </span>
                <span>→</span>
              </div>
            </NavLink>
          ))}
        </div>
      </div>

      {/* Known Limitations */}
      <div className="card" style={{ borderColor: 'var(--accent-amber-dim)' }}>
        <div className="card-title" style={{ color: 'var(--accent-amber)' }}>Known Limitations</div>
        <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
          <p style={{ marginBottom: '0.75rem' }}>
            <strong>Podman Tarballs:</strong> Docker-saved tarballs are supported as of hauler v1.3,
            but Podman-generated tarballs are not supported for the <code>store load</code> command.
          </p>
          <p style={{ marginBottom: '0.75rem' }}>
            <strong>Copy to Registry Path:</strong> When using <code>store copy</code> to copy to a registry
            path (e.g., <code>registry://example.com/my-path</code>), you must first login to the registry
            root without the path (e.g., <code>docker.io</code>, not <code>docker.com/my-path</code>).
          </p>
          <p style={{ marginBottom: '0' }}>
            <strong>Temp Directory Space:</strong> Large operations (sync, save) may require significant
            temporary space. Ensure <code>{config?.haulerTempDir || '/data/tmp'}</code> has adequate disk space.
          </p>
        </div>
      </div>
    </div>
  )
}

function Serve() {
  // Registry state
  const [registryPort, setRegistryPort] = useState(5000)
  const [registryReadonly, setRegistryReadonly] = useState(true)
  const [registryAutoTls, setRegistryAutoTls] = useState(false)
  const [registryTlsCert, setRegistryTlsCert] = useState('')
  const [registryTlsKey, setRegistryTlsKey] = useState('')
  const [registryDirectory, setRegistryDirectory] = useState('')
  const [registryConfigFile, setRegistryConfigFile] = useState('')
  const [registryShowAdvanced, setRegistryShowAdvanced] = useState(false)
  const [registryProcesses, setRegistryProcesses] = useState([])
  const [registryLoading, setRegistryLoading] = useState(true)

  // Fileserver state
  const [fileserverPort, setFileserverPort] = useState(5001)
  const [fileserverTimeout, setFileserverTimeout] = useState('')
  const [fileserverAutoTls, setFileserverAutoTls] = useState(false)
  const [fileserverTlsCert, setFileserverTlsCert] = useState('')
  const [fileserverTlsKey, setFileserverTlsKey] = useState('')
  const [fileserverDirectory, setFileserverDirectory] = useState('')
  const [fileserverShowAdvanced, setFileserverShowAdvanced] = useState(false)
  const [fileserverProcesses, setFileserverProcesses] = useState([])
  const [fileserverLoading, setFileserverLoading] = useState(true)

  // Shared UI state
  const [error, setError] = useState(null)
  const [submitting, setSubmitting] = useState(false)

  // Fetch registry processes
  const fetchRegistryProcesses = async () => {
    try {
      const res = await fetch('/api/serve/registry')
      if (res.ok) {
        const data = await res.json()
        setRegistryProcesses(data)
      }
    } catch (err) {
      console.error('Failed to fetch registry processes:', err)
    } finally {
      setRegistryLoading(false)
    }
  }

  // Fetch fileserver processes
  const fetchFileserverProcesses = async () => {
    try {
      const res = await fetch('/api/serve/fileserver')
      if (res.ok) {
        const data = await res.json()
        setFileserverProcesses(data)
      }
    } catch (err) {
      console.error('Failed to fetch fileserver processes:', err)
    } finally {
      setFileserverLoading(false)
    }
  }

  useEffect(() => {
    fetchRegistryProcesses()
    fetchFileserverProcesses()
    const interval = setInterval(() => {
      fetchRegistryProcesses()
      fetchFileserverProcesses()
    }, 5000)
    return () => clearInterval(interval)
  }, [])

  const handleStartRegistry = async (e) => {
    e.preventDefault()
    setError(null)
    setSubmitting(true)

    try {
      const requestPayload = {
        port: registryPort || 5000,
        readonly: registryReadonly,
        tlsCert: registryAutoTls ? undefined : (registryTlsCert || undefined),
        tlsKey: registryAutoTls ? undefined : (registryTlsKey || undefined),
        autoTls: registryAutoTls || undefined,
        directory: registryDirectory || undefined,
        configFile: registryConfigFile || undefined,
      }

      Object.keys(requestPayload).forEach(key => {
        if (requestPayload[key] === undefined) {
          delete requestPayload[key]
        }
      })

      const res = await fetch('/api/serve/registry', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestPayload)
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Start registry failed')
      }

      await fetchRegistryProcesses()
    } catch (err) {
      setError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  const handleStopRegistry = async (pid) => {
    setError(null)
    setSubmitting(true)

    try {
      const res = await fetch(`/api/serve/registry/${pid}`, {
        method: 'DELETE'
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Stop registry failed')
      }

      await fetchRegistryProcesses()
    } catch (err) {
      setError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  const handleStartFileserver = async (e) => {
    e.preventDefault()
    setError(null)
    setSubmitting(true)

    try {
      const requestPayload = {
        port: fileserverPort || 8080,
        timeout: fileserverTimeout ? parseInt(fileserverTimeout) : undefined,
        tlsCert: fileserverAutoTls ? undefined : (fileserverTlsCert || undefined),
        tlsKey: fileserverAutoTls ? undefined : (fileserverTlsKey || undefined),
        autoTls: fileserverAutoTls || undefined,
        directory: fileserverDirectory || undefined,
      }

      Object.keys(requestPayload).forEach(key => {
        if (requestPayload[key] === undefined) {
          delete requestPayload[key]
        }
      })

      const res = await fetch('/api/serve/fileserver', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestPayload)
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Start fileserver failed')
      }

      await fetchFileserverProcesses()
    } catch (err) {
      setError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  const handleStopFileserver = async (pid) => {
    setError(null)
    setSubmitting(true)

    try {
      const res = await fetch(`/api/serve/fileserver/${pid}`, {
        method: 'DELETE'
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Stop fileserver failed')
      }

      await fetchFileserverProcesses()
    } catch (err) {
      setError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  const getStatusBadgeClass = (status) => {
    switch (status) {
      case 'running':
        return 'badge-warning'
      case 'stopped':
        return 'badge-success'
      case 'crashed':
        return 'badge-error'
      default:
        return 'badge-info'
    }
  }

  const formatTime = (dateStr) => {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleString()
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Serve</h1>
          <p className="page-subtitle">Serve content from your store</p>
        </div>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {/* Two-column layout */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(350px, 1fr))', gap: '1.5rem' }}>

        {/* Registry Column */}
        <div className="serve-column">
          <div className="card">
            <div className="card-title" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <Package size={20} style={{ color: 'var(--accent-amber-dim)' }} />
              Registry
            </div>

            <form onSubmit={handleStartRegistry}>
              <div className="form-group">
                <label className="form-label">Port</label>
                <input
                  className="form-input"
                  type="number"
                  min="1"
                  max="65535"
                  placeholder="5000"
                  value={registryPort}
                  onChange={(e) => setRegistryPort(parseInt(e.target.value) || 5000)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Port for the registry (default: 5000)
                </div>
              </div>

              <div className="form-group">
                <label className="form-label" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <input
                    type="checkbox"
                    checked={registryReadonly}
                    onChange={(e) => setRegistryReadonly(e.target.checked)}
                    disabled={submitting}
                    style={{ width: 'auto' }}
                  />
                  <span>Readonly</span>
                </label>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Read-only mode (default: true)
                </div>
              </div>

              <div className="form-group">
                <label className="form-label" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <input
                    type="checkbox"
                    checked={registryAutoTls}
                    onChange={(e) => setRegistryAutoTls(e.target.checked)}
                    disabled={submitting}
                    style={{ width: 'auto' }}
                  />
                  <span>Enable TLS with self-signed certificate</span>
                </label>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Auto-generate and use a self-signed certificate for HTTPS
                </div>
              </div>

              <button
                type="button"
                className="btn btn-sm"
                onClick={() => setRegistryShowAdvanced(!registryShowAdvanced)}
                style={{ marginTop: '0.5rem' }}
              >
                {registryShowAdvanced ? '▼ Hide Advanced' : '▶ Show Advanced'}
              </button>

              {registryShowAdvanced && (
                <div style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid var(--border-color)' }}>
                  {!registryAutoTls && (
                    <>
                      <div className="form-group">
                        <label className="form-label">TLS Certificate Path</label>
                        <input
                          className="form-input"
                          placeholder="/path/to/cert.pem"
                          value={registryTlsCert}
                          onChange={(e) => setRegistryTlsCert(e.target.value)}
                          disabled={submitting}
                        />
                      </div>

                      <div className="form-group">
                        <label className="form-label">TLS Key Path</label>
                        <input
                          className="form-input"
                          placeholder="/path/to/key.pem"
                          value={registryTlsKey}
                          onChange={(e) => setRegistryTlsKey(e.target.value)}
                          disabled={submitting}
                        />
                      </div>
                    </>
                  )}

                  <div className="form-group">
                    <label className="form-label">Store Directory</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/store"
                      value={registryDirectory}
                      onChange={(e) => setRegistryDirectory(e.target.value)}
                      disabled={submitting}
                    />
                  </div>

                  <div className="form-group">
                    <label className="form-label">Config File</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/config.yaml"
                      value={registryConfigFile}
                      onChange={(e) => setRegistryConfigFile(e.target.value)}
                      disabled={submitting}
                    />
                  </div>
                </div>
              )}

              <button
                type="submit"
                className="btn btn-primary"
                disabled={submitting}
                style={{ width: '100%', marginTop: '1rem' }}
              >
                {submitting ? 'Starting...' : 'Start Registry'}
              </button>
            </form>
          </div>

          {/* Registry Processes */}
          <div className="card" style={{ marginTop: '1rem' }}>
            <div className="card-title">Running Processes</div>
            {registryLoading ? (
              <div style={{ color: 'var(--text-secondary)' }}>Loading...</div>
            ) : registryProcesses.length === 0 ? (
              <div style={{ color: 'var(--text-secondary)' }}>No registry processes running</div>
            ) : (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>PID</th>
                    <th>Port</th>
                    <th>Status</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {registryProcesses.map(proc => (
                    <tr key={proc.id}>
                      <td className="primary">#{proc.pid}</td>
                      <td>{proc.port}</td>
                      <td><span className={`badge ${getStatusBadgeClass(proc.status)}`}>{proc.status}</span></td>
                      <td>
                        {proc.status === 'running' && (
                          <button
                            className="btn btn-sm"
                            onClick={() => handleStopRegistry(proc.pid)}
                            disabled={submitting}
                          >
                            Stop
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          {/* Registry Access Info */}
          {registryProcesses.length > 0 && (() => {
            const proc = registryProcesses.find(p => p.status === 'running') || registryProcesses[0]
            const hasTls = proc?.args?.autoTls || proc?.args?.tlsCert
            const protocol = hasTls ? 'https' : 'http'
            const port = proc?.port || registryPort
            return (
              <div className="card" style={{ marginTop: '1rem' }}>
                <div className="card-title">Access Info</div>
                <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
                  <p style={{ marginBottom: '0.5rem' }}>Access the registry at:</p>
                  <code style={{ display: 'block', padding: '0.5rem', backgroundColor: 'var(--bg-primary)', borderRadius: '4px' }}>
                    {protocol}://localhost:{port}
                  </code>
                  <p style={{ marginTop: '0.75rem', marginBottom: '0' }}>
                    Pull images with: <code>docker pull {protocol}://localhost:{port}/myimage:tag</code>
                  </p>
                  {hasTls && (
                    <p style={{ marginTop: '0.5rem', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                      Note: Using self-signed certificate. Browser may show a security warning.
                    </p>
                  )}
                </div>
              </div>
            )
          })()}
        </div>

        {/* Fileserver Column */}
        <div className="serve-column">
          <div className="card">
            <div className="card-title" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <Folder size={20} style={{ color: 'var(--accent-amber-dim)' }} />
              Fileserver
            </div>

            <form onSubmit={handleStartFileserver}>
              <div className="form-group">
                <label className="form-label">Port</label>
                <input
                  className="form-input"
                  type="number"
                  min="1"
                  max="65535"
                  placeholder="8080"
                  value={fileserverPort}
                  onChange={(e) => setFileserverPort(parseInt(e.target.value) || 8080)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Port for the fileserver (default: 8080)
                </div>
              </div>

              <div className="form-group">
                <label className="form-label">Timeout</label>
                <input
                  className="form-input"
                  type="number"
                  min="0"
                  placeholder="0"
                  value={fileserverTimeout}
                  onChange={(e) => setFileserverTimeout(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Timeout in seconds (default: 0 / no timeout)
                </div>
              </div>

              <div className="form-group">
                <label className="form-label" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <input
                    type="checkbox"
                    checked={fileserverAutoTls}
                    onChange={(e) => setFileserverAutoTls(e.target.checked)}
                    disabled={submitting}
                    style={{ width: 'auto' }}
                  />
                  <span>Enable TLS with self-signed certificate</span>
                </label>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Auto-generate and use a self-signed certificate for HTTPS
                </div>
              </div>

              <button
                type="button"
                className="btn btn-sm"
                onClick={() => setFileserverShowAdvanced(!fileserverShowAdvanced)}
                style={{ marginTop: '0.5rem' }}
              >
                {fileserverShowAdvanced ? '▼ Hide Advanced' : '▶ Show Advanced'}
              </button>

              {fileserverShowAdvanced && (
                <div style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid var(--border-color)' }}>
                  {!fileserverAutoTls && (
                    <>
                      <div className="form-group">
                        <label className="form-label">TLS Certificate Path</label>
                        <input
                          className="form-input"
                          placeholder="/path/to/cert.pem"
                          value={fileserverTlsCert}
                          onChange={(e) => setFileserverTlsCert(e.target.value)}
                          disabled={submitting}
                        />
                      </div>

                      <div className="form-group">
                        <label className="form-label">TLS Key Path</label>
                        <input
                          className="form-input"
                          placeholder="/path/to/key.pem"
                          value={fileserverTlsKey}
                          onChange={(e) => setFileserverTlsKey(e.target.value)}
                          disabled={submitting}
                        />
                      </div>
                    </>
                  )}

                  <div className="form-group">
                    <label className="form-label">Store Directory</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/store"
                      value={fileserverDirectory}
                      onChange={(e) => setFileserverDirectory(e.target.value)}
                      disabled={submitting}
                    />
                  </div>
                </div>
              )}

              <button
                type="submit"
                className="btn btn-primary"
                disabled={submitting}
                style={{ width: '100%', marginTop: '1rem' }}
              >
                {submitting ? 'Starting...' : 'Start Fileserver'}
              </button>
            </form>
          </div>

          {/* Fileserver Processes */}
          <div className="card" style={{ marginTop: '1rem' }}>
            <div className="card-title">Running Processes</div>
            {fileserverLoading ? (
              <div style={{ color: 'var(--text-secondary)' }}>Loading...</div>
            ) : fileserverProcesses.length === 0 ? (
              <div style={{ color: 'var(--text-secondary)' }}>No fileserver processes running</div>
            ) : (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>PID</th>
                    <th>Port</th>
                    <th>Status</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {fileserverProcesses.map(proc => (
                    <tr key={proc.id}>
                      <td className="primary">#{proc.pid}</td>
                      <td>{proc.port}</td>
                      <td><span className={`badge ${getStatusBadgeClass(proc.status)}`}>{proc.status}</span></td>
                      <td>
                        {proc.status === 'running' && (
                          <button
                            className="btn btn-sm"
                            onClick={() => handleStopFileserver(proc.pid)}
                            disabled={submitting}
                          >
                            Stop
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          {/* Fileserver Access Info */}
          {fileserverProcesses.length > 0 && (() => {
            const proc = fileserverProcesses.find(p => p.status === 'running') || fileserverProcesses[0]
            const hasTls = proc?.args?.autoTls || proc?.args?.tlsCert
            const protocol = hasTls ? 'https' : 'http'
            const port = proc?.port || fileserverPort
            return (
              <div className="card" style={{ marginTop: '1rem' }}>
                <div className="card-title">Access Info</div>
                <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
                  <p style={{ marginBottom: '0.5rem' }}>Access the fileserver at:</p>
                  <code style={{ display: 'block', padding: '0.5rem', backgroundColor: 'var(--bg-primary)', borderRadius: '4px' }}>
                    {protocol}://localhost:{port}
                  </code>
                  <p style={{ marginTop: '0.75rem', marginBottom: '0' }}>
                    Download files with: <code>curl {protocol}://localhost:{port}/&lt;file-path&gt;</code>
                  </p>
                  {hasTls && (
                    <p style={{ marginTop: '0.5rem', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                      Note: Using self-signed certificate. Use <code>-k</code> flag with curl to bypass certificate verification.
                    </p>
                  )}
                </div>
              </div>
            )
          })()}
        </div>
      </div>

      {/* About Section */}
      <div className="card" style={{ marginTop: '1.5rem' }}>
        <div className="card-title">About Serve</div>
        <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
          <p style={{ marginBottom: '0.75rem' }}>
            <strong>Registry:</strong> Starts an embedded container registry that serves images from your hauler store.
            Useful for air-gapped environments or local testing.
          </p>
          <p>
            <strong>Fileserver:</strong> Starts an embedded HTTP file server that serves charts, files, and other content
            from your hauler store via HTTP.
          </p>
        </div>
      </div>
    </div>
  )
}

function RegistryLogin() {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()

  const [registry, setRegistry] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [successMessage, setSuccessMessage] = useState(null)
  const [registryInfo, setRegistryInfo] = useState(null)

  useEffect(() => {
    fetch('/api/registry/info')
      .then(res => res.json())
      .then(data => setRegistryInfo(data))
      .catch(() => setRegistryInfo(null))
  }, [])

  const handleLogin = async (e) => {
    e.preventDefault()
    setError(null)
    setSuccessMessage(null)
    setLoading(true)

    try {
      const res = await fetch('/api/registry/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ registry, username, password })
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Login request failed')
      }

      setSuccessMessage(`Login job started for ${registry}`)
      setRegistry('')
      setUsername('')
      setPassword('')

      // Refresh jobs list and navigate to job detail
      await fetchJobs()
      navigate(`/jobs/${data.jobId}`)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleLogout = async (registryUrl) => {
    setError(null)
    setSuccessMessage(null)
    setLoading(true)

    try {
      const res = await fetch('/api/registry/logout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ registry: registryUrl })
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Logout request failed')
      }

      setSuccessMessage(`Logout job started for ${registryUrl}`)

      // Refresh jobs list and navigate to job detail
      await fetchJobs()
      navigate(`/jobs/${data.jobId}`)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Registry Login</h1>
          <p className="page-subtitle">Manage container registry credentials</p>
        </div>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {successMessage && (
        <div className="card" style={{ borderColor: 'var(--accent-green)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-green)' }}>Success</div>
          <p style={{ color: 'var(--text-secondary)' }}>{successMessage}</p>
        </div>
      )}

      <div className="card" style={{ maxWidth: '500px' }}>
        <div className="card-title">Login to Registry</div>
        <form onSubmit={handleLogin}>
          <div className="form-group">
            <label className="form-label">Registry URL</label>
            <input
              className="form-input"
              placeholder="docker.io or ghcr.io"
              value={registry}
              onChange={(e) => setRegistry(e.target.value)}
              disabled={loading}
              required
            />
          </div>
          <div className="form-group">
            <label className="form-label">Username</label>
            <input
              className="form-input"
              type="text"
              placeholder="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              disabled={loading}
              required
            />
          </div>
          <div className="form-group">
            <label className="form-label">Password</label>
            <input
              className="form-input"
              type="password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={loading}
              required
            />
          </div>
          <button type="submit" className="btn btn-primary" disabled={loading}>
            {loading ? 'Starting Login...' : 'Login'}
          </button>
        </form>
      </div>

      <div className="card" style={{ maxWidth: '500px', marginTop: '1.5rem' }}>
        <div className="card-title">Quick Logout</div>
        <form onSubmit={(e) => { e.preventDefault(); handleLogout(registry) }}>
          <div className="form-group">
            <label className="form-label">Registry URL</label>
            <input
              className="form-input"
              placeholder="docker.io"
              value={registry}
              onChange={(e) => setRegistry(e.target.value)}
              disabled={loading}
            />
          </div>
          <button type="button" className="btn" onClick={() => handleLogout(registry)} disabled={loading || !registry}>
            Logout
          </button>
        </form>
      </div>

      {registryInfo && (
        <div className="card" style={{ marginTop: '1.5rem' }}>
          <div className="card-title">About Credential Storage</div>
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
            <p>
              <strong>Note:</strong> Your password is <strong>not stored</strong> in the hauler-ui database.
              Credentials are managed by hauler and stored in the Docker configuration file.
            </p>
            <p style={{ marginTop: '0.75rem' }}>
              <strong>Storage Location:</strong> <code>{registryInfo.displayPath || registryInfo.dockerAuthPath}</code>
            </p>
            <p style={{ marginTop: '0.75rem', fontSize: '0.85rem' }}>
              Hauler uses the standard Docker auth pattern. Your credentials are encrypted and stored
              in the config.json file, which is mounted from the persistent data volume.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}

function JobHistory() {
  const { jobs, deleteAllJobs, fetchJobs } = useJobs()
  const [isDeleting, setIsDeleting] = useState(false)

  const handleClearAll = async () => {
    if (!window.confirm('Are you sure you want to delete all jobs? This cannot be undone.')) {
      return
    }
    setIsDeleting(true)
    try {
      await deleteAllJobs()
      await fetchJobs()
    } catch (err) {
      alert('Failed to clear jobs: ' + err.message)
    } finally {
      setIsDeleting(false)
    }
  }

  const formatTime = (dateStr) => {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleString()
  }

  const formatDuration = (started, completed) => {
    if (!started || !completed) return '-'
    const start = new Date(started)
    const end = new Date(completed)
    const ms = end - start
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`
  }

  return (
    <div className="page">
      <div className="page-header">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%' }}>
          <div>
            <h1 className="page-title">Job History</h1>
            <p className="page-subtitle">View and manage background jobs</p>
          </div>
          {jobs.length > 0 && (
            <button
              className="btn btn-danger"
              onClick={handleClearAll}
              disabled={isDeleting}
              style={{ backgroundColor: 'var(--accent-red)', borderColor: 'var(--accent-red)' }}
            >
              {isDeleting ? 'Deleting...' : 'Clear All Jobs'}
            </button>
          )}
        </div>
      </div>

      {jobs.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state-icon"><Inbox size={48} style={{ color: 'var(--text-muted)' }} /></div>
          <div className="empty-state-text">No jobs yet</div>
        </div>
      ) : (
        <table className="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Command</th>
              <th>Status</th>
              <th>Duration</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {jobs.map(job => (
              <tr key={job.id}>
                <td className="primary">#{job.id}</td>
                <td>
                  <code>{job.command} {(job.args || []).join(' ')}</code>
                </td>
                <td><StatusBadge status={job.status} /></td>
                <td>{formatDuration(job.startedAt, job.completedAt)}</td>
                <td style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                  {formatTime(job.createdAt)}
                </td>
                <td>
                  <NavLink to={`/jobs/${job.id}`} className="btn btn-sm">View</NavLink>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function JobDetail() {
  const location = useLocation()
  const jobId = location.pathname.split('/').pop()
  const [job, setJob] = useState(null)
  const [logs, setLogs] = useState([])
  const [error, setError] = useState(null)

  // Normalize job data from API (handles both capitalized and lowercase fields)
  const normalizeJob = (data) => ({
    id: data.ID || data.id,
    command: data.Command || data.command,
    args: data.Args || data.args || [],
    status: (data.Status || data.status || '').toLowerCase(),
    exitCode: data.ExitCode ?? data.exitCode,
    startedAt: data.StartedAt || data.startedAt,
    completedAt: data.CompletedAt || data.completedAt,
    createdAt: data.CreatedAt || data.createdAt,
    result: data.Result?.String || data.result,
    envOverrides: data.EnvOverrides || data.envOverrides
  })

  useEffect(() => {
    let eventSource = null
    let pollInterval = null
    let sseActive = false

    // Polling function to refresh job status
    const pollJobStatus = async () => {
      try {
        const res = await fetch(`/api/jobs/${jobId}`)
        if (res.ok) {
          const data = await res.json()
          const normalizedJob = normalizeJob(data)
          setJob(normalizedJob)
          // Stop polling if job is complete
          if (normalizedJob.status === 'succeeded' || normalizedJob.status === 'failed') {
            if (pollInterval) {
              clearInterval(pollInterval)
              pollInterval = null
            }
          }
        }
      } catch (err) {
        console.error('Failed to poll job status:', err)
      }
    }

    // Fetch job details
    fetch(`/api/jobs/${jobId}`)
      .then(res => {
        if (!res.ok) throw new Error('Job not found')
        return res.json()
      })
      .then(data => {
        const normalizedJob = normalizeJob(data)
        setJob(normalizedJob)
        // Start polling if job is not complete
        if (normalizedJob.status !== 'succeeded' && normalizedJob.status !== 'failed') {
          pollInterval = setInterval(pollJobStatus, 2000)
        }
      })
      .catch(err => setError(err.message))

    // Fetch initial logs
    fetch(`/api/jobs/${jobId}/logs`)
      .then(res => res.json())
      .then(data => setLogs(Array.isArray(data) ? data : []))
      .catch(err => console.error('Failed to fetch logs:', err))

    // Set up SSE for streaming
    eventSource = new EventSource(`/api/jobs/${jobId}/stream`)

    eventSource.addEventListener('log', (e) => {
      try {
        const data = JSON.parse(e.data)
        setLogs(prev => [...prev, data])
      } catch (err) {
        console.error('Failed to parse log event:', err)
      }
    })

    eventSource.addEventListener('state', (e) => {
      try {
        const data = JSON.parse(e.data)
        setJob(normalizeJob(data))
      } catch (err) {
        console.error('Failed to parse state event:', err)
      }
    })

    eventSource.addEventListener('complete', (e) => {
      try {
        const data = JSON.parse(e.data)
        setJob(normalizeJob(data))
      } catch (err) {
        console.error('Failed to parse complete event:', err)
      }
      if (eventSource) {
        eventSource.close()
        eventSource = null
      }
      if (pollInterval) {
        clearInterval(pollInterval)
        pollInterval = null
      }
    })

    eventSource.onopen = () => {
      sseActive = true
      // If SSE is working well, we can reduce polling frequency or stop it
      if (pollInterval && !sseActive) {
        clearInterval(pollInterval)
        pollInterval = setInterval(pollJobStatus, 5000)
      }
    }

    eventSource.onerror = () => {
      // Don't close immediately, just note the error
      // Polling will keep the status updated
      if (eventSource) {
        eventSource.close()
        eventSource = null
      }
    }

    return () => {
      if (eventSource) {
        eventSource.close()
      }
      if (pollInterval) {
        clearInterval(pollInterval)
      }
    }
  }, [jobId])

  if (error) {
    return (
      <div className="page">
        <div className="card" style={{ borderColor: 'var(--accent-red)' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
          <NavLink to="/jobs" className="btn btn-sm">Back to Jobs</NavLink>
        </div>
      </div>
    )
  }

  if (!job) {
    return (
      <div className="page">
        <div className="loading">Loading job details...</div>
      </div>
    )
  }

  const formatCommand = () => {
    const args = (job.args || []).map(a => a.includes(' ') ? `"${a}"` : a).join(' ')
    return `${job.command} ${args}`
  }

  const formatExitInfo = () => {
    if (job.status === 'succeeded') {
      return (
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <Check size={18} style={{ color: 'var(--accent-green)' }} />
          <span>Exit code: 0</span>
        </div>
      )
    }
    if (job.status === 'failed' && job.exitCode !== undefined) {
      return (
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <X size={18} style={{ color: 'var(--accent-red)' }} />
          <span>Exit code: {job.exitCode}</span>
        </div>
      )
    }
    return null
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Job #{job.id}</h1>
          <p className="page-subtitle">
            <StatusBadge status={job.status} />
            <span style={{ marginLeft: '0.75rem', color: 'var(--text-muted)' }}>
              {new Date(job.createdAt).toLocaleString()}
            </span>
          </p>
        </div>
        <NavLink to="/jobs" className="btn">← Back</NavLink>
      </div>

      <div className="card">
        <div className="card-title">Command</div>
        <code style={{
          display: 'block',
          padding: '0.75rem',
          backgroundColor: 'var(--bg-primary)',
          border: '1px solid var(--border-color)',
          borderRadius: '2px',
          fontFamily: 'var(--font-mono)',
          fontSize: '0.85rem',
          color: 'var(--accent-amber)'
        }}>
          {formatCommand()}
        </code>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem' }}>
        <div className="card">
          <div className="card-title">Status</div>
          <StatusBadge status={job.status} />
        </div>
        <div className="card">
          <div className="card-title">Created</div>
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
            {new Date(job.createdAt).toLocaleString()}
          </div>
        </div>
        <div className="card">
          <div className="card-title">Started</div>
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
            {job.startedAt ? new Date(job.startedAt).toLocaleString() : '-'}
          </div>
        </div>
        <div className="card">
          <div className="card-title">Completed</div>
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
            {job.completedAt ? new Date(job.completedAt).toLocaleString() : '-'}
          </div>
        </div>
      </div>

      {(job.status === 'failed' || job.status === 'succeeded') && (
        <div className={`card ${job.status === 'failed' ? 'error-card' : ''}`}>
          <div className="card-title">Result</div>
          {formatExitInfo()}
        </div>
      )}

      {/* Show download link for store save jobs */}
      {job.status === 'succeeded' && job.result && (() => {
        try {
          const result = JSON.parse(job.result)
          // Store save job result
          if (result.archivePath && result.filename) {
            return (
              <div className="card" style={{ borderColor: 'var(--accent-green)' }}>
                <div className="card-title" style={{ color: 'var(--accent-green)' }}>
                  Archive Ready
                </div>
                <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
                  <p style={{ marginBottom: '0.5rem' }}>
                    <strong>Archive path:</strong> <code>{result.archivePath}</code>
                  </p>
                  <a
                    href={`/api/downloads/${result.filename}`}
                    className="btn btn-primary"
                    download
                  >
                    Download {result.filename}
                  </a>
                </div>
              </div>
            )
          }
          // Store extract job result
          if (result.outputDir) {
            return (
              <div className="card" style={{ borderColor: 'var(--accent-green)' }}>
                <div className="card-title" style={{ color: 'var(--accent-green)' }}>
                  Extraction Complete
                </div>
                <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
                  <p style={{ marginBottom: '0.5rem' }}>
                    <strong>Output directory:</strong> <code>{result.outputDir}</code>
                  </p>
                </div>
              </div>
            )
          }
        } catch {
          return null
        }
        return null
      })()}

      <div className="card">
        <div className="card-title">Output</div>
        <div className="terminal-output">
          {logs.length === 0 ? (
            <div style={{ color: 'var(--text-muted)' }}>No output yet...</div>
          ) : (
            logs.map((log, i) => {
              const content = typeof log === 'string' ? log : (log?.content || String(log))
              const stream = typeof log === 'object' ? log?.stream : ''
              return (
                <div key={i} className={`terminal-line ${stream || ''}`}>
                  <span className="content">{content}</span>
                </div>
              )
            })
          )}
          {job.status === 'running' && (
            <div className="terminal-line">
              <span className="content" style={{ color: 'var(--accent-amber)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <Loader size={14} className="spin" />
                Loading...
              </span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// === Main App ===

function TopBar() {
  const { logout, authEnabled } = useAuth()
  const navigate = useNavigate()

  const handleLogout = async () => {
    await logout()
    navigate('/login')
  }

  return (
    <div className="top-bar">
      <div className="top-bar-left">
        <span style={{ color: 'var(--accent-amber-dim)' }}>$</span> hauler-ui
      </div>
      <div className="top-bar-right">
        <JobIndicator />
        {authEnabled && (
          <button className="btn btn-sm" onClick={handleLogout} style={{ marginLeft: '0.5rem' }}>
            Logout
          </button>
        )}
      </div>
    </div>
  )
}

function ProtectedRoute({ children }) {
  const { isAuthenticated, authEnabled, loading } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    if (!loading) {
      if (authEnabled && !isAuthenticated) {
        navigate('/login')
      }
    }
  }, [isAuthenticated, authEnabled, loading, navigate])

  if (loading) {
    return (
      <div className="page">
        <div className="loading">Loading...</div>
      </div>
    )
  }

  if (authEnabled && !isAuthenticated) {
    return null // Will redirect via useEffect
  }

  return children
}

function App() {
  return (
    <Router>
      <AuthProvider>
        <ModalProvider>
          <JobsProvider>
            <Routes>
              <Route path="/login" element={<Login />} />
              <Route path="*" element={
                <ProtectedRoute>
                  <div className="App">
                    <Sidebar />
                    <div className="main-wrapper">
                      <TopBar />
                      <main className="main-content">
                        <Routes>
                          <Route path="/" element={<Dashboard />} />
                          <Route path="/store" element={<Store />} />
                          <Route path="/store/add" element={<StoreAddImage />} />
                          <Route path="/store/add-chart" element={<StoreAddChart />} />
                          <Route path="/store/add-file" element={<StoreAddFile />} />
                          <Route path="/store/sync" element={<StoreSync />} />
                          <Route path="/store/sync/:manifestId" element={<StoreSync />} />
                          <Route path="/store/save" element={<StoreSave />} />
                          <Route path="/store/load" element={<StoreLoad />} />
                          <Route path="/store/extract" element={<StoreExtract />} />
                          <Route path="/store/copy" element={<StoreCopy />} />
                          <Route path="/store/remove" element={<StoreRemove />} />
                          <Route path="/store/contents" element={<StoreContents />} />
                          <Route path="/manifests" element={<Manifests />} />
                          <Route path="/hauls" element={<Hauls />} />
                          <Route path="/serve" element={<Serve />} />
                          <Route path="/registry" element={<RegistryLogin />} />
                          <Route path="/settings" element={<Settings />} />
                          <Route path="/jobs" element={<JobHistory />} />
                          <Route path="/jobs/:id" element={<JobDetail />} />
                        </Routes>
                      </main>
                    </div>
                  </div>
                </ProtectedRoute>
              } />
            </Routes>
          </JobsProvider>
        </ModalProvider>
      </AuthProvider>
    </Router>
  )
}

function Settings() {
  const [config, setConfig] = useState(null)
  const [settings, setSettings] = useState({
    logLevel: 'info',
    retries: '0',
    ignoreErrors: 'false',
    defaultPlatform: '',
    defaultKeyPath: '',
    tempDir: ''
  })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [success, setSuccess] = useState(false)

  // Fetch both config and settings on mount
  useEffect(() => {
    fetch('/api/config')
      .then(res => res.json())
      .then(data => setConfig(data))
      .catch(() => setConfig({}))

    fetch('/api/settings')
      .then(res => res.json())
      .then(data => {
        setSettings({
          logLevel: data.logLevel || 'info',
          retries: data.retries || '0',
          ignoreErrors: data.ignoreErrors || 'false',
          defaultPlatform: data.defaultPlatform || '',
          defaultKeyPath: data.defaultKeyPath || '',
          tempDir: data.tempDir || ''
        })
      })
      .catch(() => {})
  }, [])

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError(null)
    setSuccess(false)
    setLoading(true)

    try {
      const res = await fetch('/api/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings)
      })

      if (!res.ok) {
        throw new Error('Failed to update settings')
      }

      setSuccess(true)
      setTimeout(() => setSuccess(false), 3000)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Settings</h1>
          <p className="page-subtitle">Global hauler flags and defaults</p>
        </div>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {success && (
        <div className="card" style={{ borderColor: 'var(--accent-green)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-green)' }}>Success</div>
          <p style={{ color: 'var(--text-secondary)' }}>Settings updated successfully</p>
        </div>
      )}

      <div className="card">
        <div className="card-title">Global Settings</div>
        <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', marginBottom: '1rem', lineHeight: '1.5' }}>
          These settings are applied to every hauler job execution. Values are stored in the database
          and can be overridden by environment variables.
        </p>

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">
              Log Level
              <span style={{ color: 'var(--text-muted)', fontWeight: 'normal', marginLeft: '0.5rem' }}>
                (HAULER_LOG_LEVEL)
              </span>
            </label>
            <select
              className="form-select"
              value={settings.logLevel}
              onChange={(e) => setSettings({ ...settings, logLevel: e.target.value })}
              disabled={loading}
            >
              <option value="debug">Debug</option>
              <option value="info">Info</option>
              <option value="warn">Warn</option>
              <option value="error">Error</option>
            </select>
            <p style={{ color: 'var(--text-muted)', fontSize: '0.75rem', marginTop: '0.25rem' }}>
              Controls the verbosity of hauler output. Default: info
            </p>
          </div>

          <div className="form-group">
            <label className="form-label">
              Retries
              <span style={{ color: 'var(--text-muted)', fontWeight: 'normal', marginLeft: '0.5rem' }}>
                (HAULER_RETRIES)
              </span>
            </label>
            <input
              className="form-input"
              type="number"
              min="0"
              max="10"
              value={settings.retries}
              onChange={(e) => setSettings({ ...settings, retries: e.target.value })}
              disabled={loading}
            />
            <p style={{ color: 'var(--text-muted)', fontSize: '0.75rem', marginTop: '0.25rem' }}>
              Number of times to retry failed operations. Default: 0
            </p>
          </div>

          <div className="form-group">
            <label className="form-label">
              Ignore Errors
              <span style={{ color: 'var(--text-muted)', fontWeight: 'normal', marginLeft: '0.5rem' }}>
                (HAULER_IGNORE_ERRORS)
              </span>
            </label>
            <select
              className="form-select"
              value={settings.ignoreErrors}
              onChange={(e) => setSettings({ ...settings, ignoreErrors: e.target.value })}
              disabled={loading}
            >
              <option value="false">False (stop on errors)</option>
              <option value="true">True (continue on errors)</option>
            </select>
            <p style={{ color: 'var(--text-muted)', fontSize: '0.75rem', marginTop: '0.25rem' }}>
              Continue operations even when individual items fail. Default: false
            </p>
          </div>

          <div className="form-group">
            <label className="form-label">
              Default Platform
              <span style={{ color: 'var(--text-muted)', fontWeight: 'normal', marginLeft: '0.5rem' }}>
                (HAULER_DEFAULT_PLATFORM)
              </span>
            </label>
            <input
              className="form-input"
              type="text"
              placeholder="linux/amd64 or linux/arm64"
              value={settings.defaultPlatform}
              onChange={(e) => setSettings({ ...settings, defaultPlatform: e.target.value })}
              disabled={loading}
            />
            <p style={{ color: 'var(--text-muted)', fontSize: '0.75rem', marginTop: '0.25rem' }}>
              Default platform for multi-platform operations (e.g., linux/amd64). Leave empty for auto-detection.
            </p>
          </div>

          <div className="form-group">
            <label className="form-label">
              Default Key Path
              <span style={{ color: 'var(--text-muted)', fontWeight: 'normal', marginLeft: '0.5rem' }}>
                (HAULER_KEY_PATH)
              </span>
            </label>
            <input
              className="form-input"
              type="text"
              placeholder="/path/to/cosign.key"
              value={settings.defaultKeyPath}
              onChange={(e) => setSettings({ ...settings, defaultKeyPath: e.target.value })}
              disabled={loading}
            />
            <p style={{ color: 'var(--text-muted)', fontSize: '0.75rem', marginTop: '0.25rem' }}>
              Default path to cosign private key for signature verification.
            </p>
          </div>

          <div className="form-group">
            <label className="form-label">
              Temp Directory
              <span style={{ color: 'var(--text-muted)', fontWeight: 'normal', marginLeft: '0.5rem' }}>
                (HAULER_TEMP_DIR)
              </span>
            </label>
            <input
              className="form-input"
              type="text"
              placeholder="/data/tmp"
              value={settings.tempDir}
              onChange={(e) => setSettings({ ...settings, tempDir: e.target.value })}
              disabled={loading}
            />
            <p style={{ color: 'var(--text-muted)', fontSize: '0.75rem', marginTop: '0.25rem' }}>
              Directory for temporary files during operations. Default: /data/tmp
            </p>
          </div>

          <div style={{ marginTop: '1.5rem', paddingTop: '1rem', borderTop: '1px solid var(--border-color)' }}>
            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? 'Saving...' : 'Save Settings'}
            </button>
          </div>
        </form>
      </div>

      {config && (
        <div className="card">
          <div className="card-title">System Paths</div>
          <table className="data-table">
            <tbody>
              <tr>
                <td style={{ width: '150px' }}>Store Directory</td>
                <td className="primary">
                  <code>{config.haulerStoreDir || '-'}</code>
                </td>
                <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  {config.haulerStoreEnv || 'HAULER_STORE_DIR'}
                </td>
              </tr>
              <tr>
                <td>Hauler Directory</td>
                <td className="primary">
                  <code>{config.haulerDir || '-'}</code>
                </td>
                <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  {config.haulerDirEnv || 'HAULER_DIR'}
                </td>
              </tr>
              <tr>
                <td>Database Path</td>
                <td className="primary">
                  <code>{config.databasePath || '-'}</code>
                </td>
                <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  {config.databasePathEnv || 'DATABASE_PATH'}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      )}

      <div className="card" style={{ borderColor: 'var(--accent-amber-dim)' }}>
        <div className="card-title" style={{ color: 'var(--accent-amber)' }}>About Settings</div>
        <div style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', lineHeight: '1.6' }}>
          <p style={{ marginBottom: '0.5rem' }}>
            Settings stored in the database are applied as defaults to every hauler job execution.
            These values can be overridden by setting the corresponding environment variable on the
            hauler-ui container.
          </p>
          <p>
            Environment variables take precedence over database settings. To reset a setting to its
            default value, clear the field and save.
          </p>
        </div>
      </div>
    </div>
  )
}

export default App
