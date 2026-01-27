import { useState, useEffect, createContext, useContext, useCallback } from 'react'
import { HashRouter as Router, Routes, Route, NavLink, useNavigate, useLocation } from 'react-router-dom'
import StoreAddImage from './pages/StoreAddImage.jsx'
import StoreAddChart from './pages/StoreAddChart.jsx'
import StoreAddFile from './pages/StoreAddFile.jsx'
import StoreSync from './pages/StoreSync.jsx'
import StoreSave from './pages/StoreSave.jsx'
import StoreLoad from './pages/StoreLoad.jsx'
import StoreExtract from './pages/StoreExtract.jsx'
import StoreCopy from './pages/StoreCopy.jsx'
import Manifests from './pages/Manifests.jsx'
import './App.css'

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

  const fetchJobs = useCallback(async () => {
    try {
      const res = await fetch('/api/jobs')
      if (res.ok) {
        const data = await res.json()
        setJobs(data)
        const running = data.filter(j => j.status === 'running' || j.status === 'queued').length
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

  return (
    <JobsContext.Provider value={{ jobs, runningJobCount, fetchJobs, createJob }}>
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
        { path: '/manifests', label: 'Manifests' },
        { path: '/hauls', label: 'Hauls' }
      ]
    },
    {
      title: 'Operations',
      items: [
        { path: '/serve', label: 'Serve' },
        { path: '/copy', label: 'Copy/Export' },
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
          <div className="sidebar-brand">hauler-ui</div>
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
                  onClick={() => setIsOpen(false)}
                >
                  {item.label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
      </aside>
    </>
  )
}

// === Pages ===

function Dashboard() {
  const [health, setHealth] = useState(null)

  useEffect(() => {
    fetch('/healthz')
      .then(res => res.json())
      .then(data => setHealth(data))
      .catch(() => setHealth({ status: 'error' }))
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
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <StatusBadge status={health.status === 'ok' ? 'succeeded' : 'failed'} />
            <span style={{ color: 'var(--text-secondary)' }}>
              Backend: {health.status}
            </span>
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
  const [capabilities, setCapabilities] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    // Fetch config
    fetch('/api/config')
      .then(res => res.json())
      .then(data => setConfig(data))
      .catch(err => setError('Failed to load config: ' + err.message))

    // Fetch capabilities
    fetch('/api/hauler/capabilities')
      .then(res => res.json())
      .then(data => setCapabilities(data))
      .catch(err => setError('Failed to load capabilities: ' + err.message))
  }, [])

  // Store operations mapped to routes
  const storeOperations = [
    { id: 'add-image', name: 'Add Image', description: 'Add container images to the store', icon: '🖼️', route: '/store/add' },
    { id: 'add-chart', name: 'Add Chart', description: 'Add Helm charts to the store', icon: '📊', route: '/store/add-chart' },
    { id: 'add-file', name: 'Add File', description: 'Add local files or remote URLs to the store', icon: '📄', route: '/store/add-file' },
    { id: 'sync', name: 'Sync', description: 'Sync store from manifest files', icon: '🔄', route: '/store/sync' },
    { id: 'save', name: 'Save', description: 'Package store as a portable archive', icon: '💾', route: '/store/save' },
    { id: 'load', name: 'Load', description: 'Load an archive into the store', icon: '📥', route: '/store/load' },
    { id: 'extract', name: 'Extract', description: 'Extract artifacts from the store', icon: '📤', route: '/store/extract' },
    { id: 'copy', name: 'Copy', description: 'Copy store to registry or directory', icon: '📋', route: '/store/copy' },
    { id: 'serve', name: 'Serve', description: 'Serve registry or fileserver', icon: '🌐', route: '/serve' },
    { id: 'remove', name: 'Remove', description: 'Remove artifacts from store (experimental)', icon: '🗑️', route: '/store/remove' },
  ]

  // Related pages in the app
  const relatedPages = [
    { name: 'Manifests', description: 'Create and manage hauler manifests', route: '/manifests' },
    { name: 'Hauls', description: 'Run and monitor haul operations', route: '/hauls' },
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

      {/* Hauler Version */}
      {capabilities && (
        <div className="card">
          <div className="card-title">Hauler Version</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <code style={{ fontSize: '1rem', color: 'var(--accent-amber)' }}>
              {capabilities.version.full || 'Unknown'}
            </code>
            <span style={{ color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
              Last refreshed: {new Date(capabilities.lastRefresh).toLocaleString()}
            </span>
          </div>
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
                <span style={{ fontSize: '1.5rem' }}>{op.icon}</span>
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

function Hauls() {
  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Hauls</h1>
          <p className="page-subtitle">Run and monitor haul operations</p>
        </div>
        <button className="btn btn-primary">+ New Haul</button>
      </div>
      <div className="empty-state">
        <div className="empty-state-icon">🚚</div>
        <div className="empty-state-text">Haul operations coming soon</div>
      </div>
    </div>
  )
}

function Serve() {
  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Serve</h1>
          <p className="page-subtitle">Serve content from your store</p>
        </div>
      </div>
      <div className="empty-state">
        <div className="empty-state-icon">🌐</div>
        <div className="empty-state-text">Serve operations coming soon</div>
      </div>
    </div>
  )
}

function CopyExport() {
  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Copy/Export</h1>
          <p className="page-subtitle">Copy or export store contents</p>
        </div>
      </div>
      <div className="empty-state">
        <div className="empty-state-icon">📤</div>
        <div className="empty-state-text">Copy/Export operations coming soon</div>
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
  const { jobs } = useJobs()

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
        <div>
          <h1 className="page-title">Job History</h1>
          <p className="page-subtitle">View and manage background jobs</p>
        </div>
      </div>

      {jobs.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state-icon">📭</div>
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

  useEffect(() => {
    // Fetch job details
    fetch(`/api/jobs/${jobId}`)
      .then(res => {
        if (!res.ok) throw new Error('Job not found')
        return res.json()
      })
      .then(data => setJob(data))
      .catch(err => setError(err.message))

    // Fetch initial logs
    fetch(`/api/jobs/${jobId}/logs`)
      .then(res => res.json())
      .then(data => setLogs(data))

    // Set up SSE for streaming if job is running
    const eventSource = new EventSource(`/api/jobs/${jobId}/stream`)

    eventSource.addEventListener('log', (e) => {
      const data = JSON.parse(e.data)
      setLogs(prev => [...prev, data])
    })

    eventSource.addEventListener('state', (e) => {
      const data = JSON.parse(e.data)
      setJob(data)
    })

    eventSource.addEventListener('complete', (e) => {
      const data = JSON.parse(e.data)
      setJob(data)
      eventSource.close()
    })

    eventSource.onerror = () => {
      eventSource.close()
    }

    return () => eventSource.close()
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
          <span style={{ color: 'var(--accent-green)' }}>✓</span>
          <span>Exit code: 0</span>
        </div>
      )
    }
    if (job.status === 'failed' && job.exitCode !== undefined) {
      return (
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <span style={{ color: 'var(--accent-red)' }}>✗</span>
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
            logs.map((log, i) => (
              <div key={i} className={`terminal-line ${log.stream}`}>
                <span className="content">{log.content}</span>
              </div>
            ))
          )}
          {job.status === 'running' && (
            <div className="terminal-line">
              <span className="content" style={{ color: 'var(--accent-amber)' }}>
                ▂ Loading...
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
  return (
    <div className="top-bar">
      <div className="top-bar-left">
        <span style={{ color: 'var(--accent-amber-dim)' }}>$</span> hauler-ui
      </div>
      <div className="top-bar-right">
        <JobIndicator />
      </div>
    </div>
  )
}

function App() {
  return (
    <Router>
      <JobsProvider>
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
                <Route path="/manifests" element={<Manifests />} />
                <Route path="/hauls" element={<Hauls />} />
                <Route path="/serve" element={<Serve />} />
                <Route path="/copy" element={<CopyExport />} />
                <Route path="/registry" element={<RegistryLogin />} />
                <Route path="/settings" element={<Settings />} />
                <Route path="/jobs" element={<JobHistory />} />
                <Route path="/jobs/:id" element={<JobDetail />} />
              </Routes>
            </main>
          </div>
        </div>
      </JobsProvider>
    </Router>
  )
}

function Settings() {
  const [config, setConfig] = useState(null)

  useEffect(() => {
    fetch('/api/config')
      .then(res => res.json())
      .then(data => setConfig(data))
      .catch(() => setConfig({}))
  }, [])

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Settings</h1>
          <p className="page-subtitle">System configuration</p>
        </div>
      </div>

      {config && (
        <>
          <div className="card">
            <div className="card-title">Hauler Directories</div>
            <table className="data-table">
              <tbody>
                <tr>
                  <td style={{ width: '150px' }}>Store Directory</td>
                  <td className="primary">
                    <code>{config.storeDir || '-'}</code>
                  </td>
                </tr>
                <tr>
                  <td>Config Directory</td>
                  <td className="primary">
                    <code>{config.configDir || '-'}</code>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          {config.dockerAuth && (
            <div className="card">
              <div className="card-title">Docker Authentication</div>
              <table className="data-table">
                <tbody>
                  {config.dockerAuth.auths && Object.keys(config.dockerAuth.auths).length > 0 ? (
                    Object.entries(config.dockerAuth.auths).map(([registry]) => (
                      <tr key={registry}>
                        <td style={{ width: '150px' }}>Registry</td>
                        <td className="primary"><code>{registry}</code></td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td colSpan="2" style={{ color: 'var(--text-muted)' }}>No configured registries</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  )
}

export default App
