import { useState, useEffect, useRef, useCallback } from 'react'
import { useParams, useNavigate, NavLink } from 'react-router-dom'
import { useHauls } from '../contexts/HaulContext.jsx'
import { useJobs } from '../App.jsx'
import StoreContents from './StoreContents.jsx'
import {
  Package, Image, BarChart3, FileText, RefreshCw, Save, Download, Upload,
  Clipboard, Globe, Trash2, FileArchive, ArrowLeft, Layers, UploadCloud,
} from 'lucide-react'

function formatSize(bytes) {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = bytes
  let i = 0
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(size < 10 ? 1 : 0)} ${units[i]}`
}

const TABS = [
  { id: 'overview', label: 'Overview' },
  { id: 'contents', label: 'Contents' },
  { id: 'add', label: 'Add Content' },
  { id: 'archives', label: 'Archives' },
  { id: 'serve', label: 'Serve' },
]

function HaulDetail() {
  const { id } = useParams()
  const haulId = Number(id)
  const navigate = useNavigate()
  const { hauls, activeHaulId, setActiveHaulId, refreshHauls } = useHauls()
  const [tab, setTab] = useState('overview')

  // Make this haul the active target for store operations while viewing it.
  useEffect(() => {
    if (haulId && haulId !== activeHaulId) {
      setActiveHaulId(haulId)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [haulId])

  const haul = hauls.find((h) => h.id === haulId)

  if (!haul) {
    return (
      <div className="page">
        <div className="card">
          <div className="loading">Loading haul...</div>
          <NavLink to="/hauls" className="btn btn-sm" style={{ marginTop: '1rem' }}>← Back to Hauls</NavLink>
        </div>
      </div>
    )
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <button className="btn btn-sm" onClick={() => navigate('/hauls')} title="Back to Hauls">
              <ArrowLeft size={14} />
            </button>
            <Layers size={20} style={{ color: 'var(--accent-amber)' }} />
            <h1 className="page-title" style={{ margin: 0 }}>{haul.name}</h1>
          </div>
          <p className="page-subtitle" style={{ marginTop: '0.4rem' }}>
            {haul.description || 'Isolated haul workspace'}
          </p>
        </div>
      </div>

      {/* Tabs */}
      <div style={{ display: 'flex', gap: '0.4rem', borderBottom: '1px solid var(--border-color)', marginBottom: '1.25rem', flexWrap: 'wrap' }}>
        {TABS.map((t) => (
          <button
            key={t.id}
            className="btn btn-sm"
            onClick={() => setTab(t.id)}
            style={{
              borderRadius: '6px 6px 0 0',
              borderBottom: tab === t.id ? '2px solid var(--accent-amber)' : '2px solid transparent',
              background: tab === t.id ? 'var(--bg-tertiary)' : 'transparent',
              color: tab === t.id ? 'var(--text-primary)' : 'var(--text-secondary)',
            }}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'overview' && <OverviewTab haul={haul} onGo={setTab} />}
      {tab === 'contents' && <StoreContents />}
      {tab === 'add' && <AddTab />}
      {tab === 'archives' && <ArchivesTab haul={haul} onChanged={refreshHauls} />}
      {tab === 'serve' && <ServeTab haul={haul} />}
    </div>
  )
}

function OverviewTab({ haul, onGo }) {
  const stats = [
    { label: 'Images', value: haul.imageCount || 0, icon: Image, color: 'var(--accent-blue)' },
    { label: 'Charts', value: haul.chartCount || 0, icon: BarChart3, color: 'var(--accent-green)' },
    { label: 'Files', value: haul.fileCount || 0, icon: FileText, color: 'var(--accent-amber)' },
    { label: 'Archives', value: haul.archiveCount || 0, icon: FileArchive, color: 'var(--text-secondary)' },
  ]
  return (
    <>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: '1rem', marginBottom: '1.25rem' }}>
        {stats.map((s) => (
          <div key={s.label} className="card" style={{ textAlign: 'center' }}>
            <s.icon size={22} style={{ color: s.color }} />
            <div style={{ fontSize: '1.6rem', fontWeight: 700, marginTop: '0.4rem' }}>{s.value}</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{s.label}</div>
          </div>
        ))}
      </div>

      <div className="card">
        <div className="card-title">Store Directory</div>
        <code style={{ display: 'block', padding: '0.5rem', background: 'var(--bg-primary)', borderRadius: '4px', fontSize: '0.8rem' }}>
          {haul.storeDir}
        </code>
        <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', marginTop: '0.75rem', marginBottom: 0 }}>
          This haul has its own isolated store. Operations here never affect other hauls.
        </p>
      </div>

      <div className="card">
        <div className="card-title">Quick Actions</div>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          <button className="btn" onClick={() => onGo('add')}><Image size={15} style={{ marginRight: '0.3rem' }} />Add Content</button>
          <button className="btn" onClick={() => onGo('contents')}><Package size={15} style={{ marginRight: '0.3rem' }} />View Contents</button>
          <button className="btn" onClick={() => onGo('archives')}><FileArchive size={15} style={{ marginRight: '0.3rem' }} />Build / Archives</button>
          <button className="btn" onClick={() => onGo('serve')}><Globe size={15} style={{ marginRight: '0.3rem' }} />Serve</button>
        </div>
      </div>
    </>
  )
}

function AddTab() {
  // These operations all target the active haul (set when this page loads),
  // so we simply route to the existing operation forms.
  const ops = [
    { name: 'Add Image', description: 'Add container images', icon: Image, route: '/store/add' },
    { name: 'Add Chart', description: 'Add Helm charts', icon: BarChart3, route: '/store/add-chart' },
    { name: 'Add File', description: 'Add local files or remote URLs', icon: FileText, route: '/store/add-file' },
    { name: 'Sync from Manifest', description: 'Populate from a hauler manifest', icon: RefreshCw, route: '/store/sync' },
    { name: 'Copy to Registry', description: 'Copy store to a registry or directory', icon: Clipboard, route: '/store/copy' },
    { name: 'Extract', description: 'Extract an artifact from the store', icon: Upload, route: '/store/extract' },
    { name: 'Remove', description: 'Remove artifacts from the store', icon: Trash2, route: '/store/remove' },
  ]
  return (
    <div className="card">
      <div className="card-title">Add Content to this Haul</div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: '0.75rem' }}>
        {ops.map((op) => (
          <NavLink key={op.route} to={op.route} className="operation-card" style={{ textDecoration: 'none' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
              <op.icon size={22} style={{ color: 'var(--accent-amber-dim)' }} />
              <div>
                <div style={{ fontWeight: 500, color: 'var(--text-primary)' }}>{op.name}</div>
                <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{op.description}</div>
              </div>
            </div>
          </NavLink>
        ))}
      </div>
    </div>
  )
}

function CodeBlock({ children }) {
  return (
    <pre style={{
      margin: '0.4rem 0 0', padding: '0.6rem 0.75rem', background: 'var(--bg-primary)',
      border: '1px solid var(--border-color)', borderRadius: '4px', fontSize: '0.78rem',
      overflowX: 'auto', whiteSpace: 'pre-wrap', wordBreak: 'break-all',
    }}>{children}</pre>
  )
}

function ServeTab({ haul }) {
  const [pub, setPub] = useState(null)        // { routes, registryDomain, registryPort }
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(null)
  const [hostname, setHostname] = useState('')

  const fetchPublish = useCallback(async () => {
    try {
      const res = await fetch('/api/publish')
      if (res.ok) setPub(await res.json())
    } catch { /* ignore */ }
  }, [])

  useEffect(() => { fetchPublish() }, [fetchPublish])

  const route = pub?.routes?.find((r) => r.haulId === haul.id)
  const published = !!route

  const handlePublish = async () => {
    setBusy(true); setError(null)
    try {
      const res = await fetch(`/api/publish/${haul.id}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ hostname: hostname.trim() || undefined }),
      })
      if (!res.ok) throw new Error((await res.text()) || 'Publish failed')
      await fetchPublish()
    } catch (err) { setError(err.message) } finally { setBusy(false) }
  }

  const handleUnpublish = async () => {
    setBusy(true); setError(null)
    try {
      const res = await fetch(`/api/publish/${haul.id}`, { method: 'DELETE' })
      if (!res.ok) throw new Error((await res.text()) || 'Unpublish failed')
      await fetchPublish()
    } catch (err) { setError(err.message) } finally { setBusy(false) }
  }

  const regPort = pub?.registryPort || 5000
  const portSuffix = regPort === 443 ? '' : `:${regPort}`
  const fileBase = `${window.location.origin}/h/${haul.slug}`

  return (
    <>
      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      <div className="card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem', flexWrap: 'wrap' }}>
          <div>
            <div className="card-title" style={{ border: 'none', margin: 0 }}>Publish this Haul</div>
            <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', margin: '0.25rem 0 0' }}>
              Expose this haul&apos;s registry through hauler-ui&apos;s single front door (one port, host-routed),
              and serve its files at a stable URL — no per-haul ports to manage.
            </p>
          </div>
          {published ? (
            <span className="badge badge-success" style={{ alignSelf: 'flex-start' }}>Published</span>
          ) : (
            <span className="badge" style={{ alignSelf: 'flex-start' }}>Not published</span>
          )}
        </div>

        {!published ? (
          <div style={{ marginTop: '1rem', display: 'flex', gap: '0.5rem', alignItems: 'flex-end', flexWrap: 'wrap' }}>
            <div className="form-group" style={{ marginBottom: 0, flex: 1, minWidth: '240px' }}>
              <label className="form-label">Registry hostname (optional)</label>
              <input
                className="form-input"
                placeholder={pub?.registryDomain ? `${haul.slug}.${pub.registryDomain}` : haul.slug}
                value={hostname}
                onChange={(e) => setHostname(e.target.value)}
                disabled={busy}
              />
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginTop: '0.3rem' }}>
                Clients pull from this host. Leave blank to use the default
                {pub?.registryDomain ? ` (<slug>.${pub.registryDomain})` : ' (the slug)'}.
              </div>
            </div>
            <button className="btn btn-primary" onClick={handlePublish} disabled={busy}>
              <Globe size={15} style={{ marginRight: '0.3rem' }} />
              {busy ? 'Publishing…' : 'Publish'}
            </button>
          </div>
        ) : (
          <div style={{ marginTop: '1rem' }}>
            <table className="data-table" style={{ marginBottom: '0.75rem' }}>
              <tbody>
                <tr><td style={{ width: '160px' }}>Registry host</td><td className="primary"><code>{route.hostname}</code></td></tr>
                <tr><td>Registry port</td><td><code>{regPort}</code> (shared by all published hauls)</td></tr>
                <tr><td>Internal backend</td><td style={{ color: 'var(--text-muted)' }}>127.0.0.1:{route.port} (hauler process)</td></tr>
              </tbody>
            </table>
            <button className="btn" onClick={handleUnpublish} disabled={busy} style={{ color: 'var(--accent-red)' }}>
              {busy ? 'Unpublishing…' : 'Unpublish'}
            </button>
          </div>
        )}
      </div>

      {published && (
        <div className="card">
          <div className="card-title">Client Configuration</div>

          <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Pull an image (Docker / nerdctl):</div>
          <CodeBlock>docker pull {route.hostname}{portSuffix}/&lt;repo&gt;:&lt;tag&gt;</CodeBlock>

          <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginTop: '0.85rem' }}>containerd mirror — /etc/containerd/certs.d/{route.hostname}{portSuffix}/hosts.toml:</div>
          <CodeBlock>{`server = "https://${route.hostname}${portSuffix}"

[host."https://${route.hostname}${portSuffix}"]
  capabilities = ["pull", "resolve"]`}</CodeBlock>

          <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginTop: '0.85rem' }}>Download a file from this haul:</div>
          <CodeBlock>{`curl -O ${fileBase}/<filename>
# list:  curl ${fileBase}/`}</CodeBlock>

          <p style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.75rem', marginBottom: 0 }}>
            The registry is served over HTTPS. With the default self-signed certificate, clients must trust it
            (or use insecure mode); load a CA-signed cert on the{' '}
            <NavLink to="/publish" style={{ color: 'var(--accent-amber)' }}>Publishing</NavLink> page for valid TLS.
            Requires DNS (or <code>/etc/hosts</code>) pointing <code>{route.hostname}</code> at this server.
          </p>
        </div>
      )}

      <div className="card">
        <div className="card-title">Ad-hoc Serve</div>
        <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', marginTop: 0 }}>
          Prefer a one-off registry/fileserver on a specific port? Use the manual serve controls.
        </p>
        <NavLink to="/serve" className="btn">
          <Globe size={16} style={{ marginRight: '0.3rem' }} /> Open Serve Controls
        </NavLink>
      </div>
    </>
  )
}

function ArchivesTab({ haul, onChanged }) {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()
  const fileInputRef = useRef(null)

  const [archives, setArchives] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [message, setMessage] = useState(null)
  const [building, setBuilding] = useState(false)
  const [buildName, setBuildName] = useState('')
  const [uploading, setUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [deleteConfirm, setDeleteConfirm] = useState(null)

  const fetchArchives = useCallback(async () => {
    try {
      const res = await fetch(`/api/hauls/${haul.id}/archives`)
      if (res.ok) {
        const data = await res.json()
        setArchives(data.archives || [])
        setError(null)
      }
    } catch (err) {
      setError('Failed to load archives: ' + err.message)
    } finally {
      setLoading(false)
    }
  }, [haul.id])

  useEffect(() => { fetchArchives() }, [fetchArchives])

  const flash = (msg) => { setMessage(msg); setTimeout(() => setMessage(null), 3000) }

  const handleBuild = async (e) => {
    e.preventDefault()
    setError(null)
    setBuilding(true)
    try {
      const res = await fetch('/api/store/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ haulId: haul.id, filename: buildName.trim() || undefined }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.message || 'Build failed')
      await fetchJobs()
      navigate(`/jobs/${data.jobId}`)
    } catch (err) {
      setError(err.message)
    } finally {
      setBuilding(false)
    }
  }

  const handleUpload = (e) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (!file.name.toLowerCase().endsWith('.tar.zst')) {
      setError('Only .tar.zst files are allowed')
      return
    }
    setError(null)
    setUploading(true)
    setUploadProgress(0)

    const formData = new FormData()
    formData.append('file', file)
    formData.append('haulId', String(haul.id))

    const xhr = new XMLHttpRequest()
    xhr.upload.addEventListener('progress', (ev) => {
      if (ev.lengthComputable) setUploadProgress((ev.loaded / ev.total) * 100)
    })
    xhr.addEventListener('load', async () => {
      setUploading(false)
      if (fileInputRef.current) fileInputRef.current.value = ''
      if (xhr.status === 200 || xhr.status === 202) {
        flash(`Imported ${file.name} — loading into store`)
        const data = JSON.parse(xhr.responseText || '{}')
        await fetchArchives()
        await fetchJobs()
        if (data.jobId) navigate(`/jobs/${data.jobId}`)
      } else {
        let msg = 'Upload failed'
        try { msg = JSON.parse(xhr.responseText).message || msg } catch { msg = xhr.responseText || msg }
        setError(msg)
      }
    })
    xhr.addEventListener('error', () => { setUploading(false); setError('Network error during upload') })
    xhr.open('POST', '/api/store/import')
    xhr.send(formData)
  }

  const handleLoad = async (name) => {
    setError(null)
    try {
      const res = await fetch('/api/store/load', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ haulId: haul.id, filenames: [name], clear: false }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.message || 'Load failed')
      await fetchJobs()
      navigate(`/jobs/${data.jobId}`)
    } catch (err) {
      setError(err.message)
    }
  }

  const handleDelete = async (name) => {
    setError(null)
    try {
      const res = await fetch(`/api/hauls/${haul.id}/archives/${name}`, { method: 'DELETE' })
      if (!res.ok) throw new Error('Delete failed')
      flash(`Deleted ${name}`)
      setDeleteConfirm(null)
      await fetchArchives()
      onChanged && onChanged()
    } catch (err) {
      setError(err.message)
    }
  }

  return (
    <>
      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}
      {message && (
        <div className="card" style={{ borderColor: 'var(--accent-green)', marginBottom: '1rem' }}>
          <p style={{ color: 'var(--text-secondary)', margin: 0 }}>{message}</p>
        </div>
      )}

      <div className="card">
        <div className="card-title">Build Archive</div>
        <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', marginTop: 0 }}>
          Package this haul&apos;s store into a portable <code>.tar.zst</code> archive for air-gapped transfer.
        </p>
        <form onSubmit={handleBuild} style={{ display: 'flex', gap: '0.5rem', alignItems: 'flex-end', flexWrap: 'wrap' }}>
          <div className="form-group" style={{ marginBottom: 0, flex: 1, minWidth: '220px' }}>
            <label className="form-label">Filename (optional)</label>
            <input
              className="form-input"
              placeholder={`${haul.slug}.tar.zst`}
              value={buildName}
              onChange={(e) => setBuildName(e.target.value)}
              disabled={building}
            />
          </div>
          <button type="submit" className="btn btn-primary" disabled={building}>
            <Save size={15} style={{ marginRight: '0.3rem' }} />
            {building ? 'Starting...' : 'Build Archive'}
          </button>
          <button type="button" className="btn" onClick={() => fileInputRef.current?.click()} disabled={uploading}>
            <UploadCloud size={15} style={{ marginRight: '0.3rem' }} />
            {uploading ? 'Uploading...' : 'Import Archive'}
          </button>
          <input ref={fileInputRef} type="file" accept=".tar.zst" onChange={handleUpload} style={{ display: 'none' }} />
        </form>
        {uploading && (
          <div style={{ marginTop: '0.75rem' }}>
            <div style={{ height: '4px', background: 'var(--border-color)', borderRadius: '2px', overflow: 'hidden' }}>
              <div style={{ height: '100%', width: `${uploadProgress}%`, background: 'var(--accent-green)', transition: 'width 0.2s' }} />
            </div>
            <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>{uploadProgress.toFixed(0)}%</div>
          </div>
        )}
      </div>

      <div className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div className="card-title" style={{ border: 'none', margin: 0 }}>Archives</div>
          <button className="btn btn-sm" onClick={() => { setLoading(true); fetchArchives() }}>
            <RefreshCw size={14} className={loading ? 'spin' : ''} />
          </button>
        </div>
        {loading && archives.length === 0 ? (
          <div className="loading">Loading archives...</div>
        ) : archives.length === 0 ? (
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', padding: '0.5rem 0' }}>
            No archives yet. Build one above, or import an existing <code>.tar.zst</code>.
          </div>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th style={{ width: '40px' }}></th>
                <th>Filename</th>
                <th style={{ width: '100px' }}>Size</th>
                <th style={{ width: '170px' }}>Created</th>
                <th style={{ width: '230px' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {archives.map((a) => (
                <tr key={a.name}>
                  <td><FileArchive size={16} style={{ color: 'var(--accent-amber)' }} /></td>
                  <td className="primary"><span style={{ fontWeight: 500 }}>{a.name}</span></td>
                  <td style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>{formatSize(a.size)}</td>
                  <td style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{new Date(a.modified).toLocaleString()}</td>
                  <td>
                    <div style={{ display: 'flex', gap: '0.25rem' }}>
                      <a className="btn btn-sm" href={`/api/hauls/${haul.id}/archives/${a.name}`} download title="Download">
                        <Download size={14} />
                      </a>
                      <button className="btn btn-sm" onClick={() => handleLoad(a.name)} title="Load into store" style={{ color: 'var(--accent-green)' }}>
                        <Upload size={14} />
                      </button>
                      {deleteConfirm === a.name ? (
                        <>
                          <button className="btn btn-sm" onClick={() => setDeleteConfirm(null)}>Cancel</button>
                          <button
                            className="btn btn-sm"
                            onClick={() => handleDelete(a.name)}
                            style={{ backgroundColor: 'var(--accent-red)', borderColor: 'var(--accent-red)' }}
                          >
                            <Trash2 size={14} />
                          </button>
                        </>
                      ) : (
                        <button className="btn btn-sm" onClick={() => setDeleteConfirm(a.name)} title="Delete" style={{ color: 'var(--accent-red)' }}>
                          <Trash2 size={14} />
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  )
}

export default HaulDetail
