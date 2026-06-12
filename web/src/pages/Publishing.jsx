import { useState, useEffect, useCallback } from 'react'
import { NavLink } from 'react-router-dom'
import { Globe, ShieldCheck, ShieldAlert, RefreshCw, Trash2 } from 'lucide-react'

function Publishing() {
  const [data, setData] = useState(null) // { routes, registryDomain, registryPort }
  const [tls, setTls] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [message, setMessage] = useState(null)

  // TLS upload form
  const [certPem, setCertPem] = useState('')
  const [keyPem, setKeyPem] = useState('')
  const [savingTls, setSavingTls] = useState(false)

  const fetchAll = useCallback(async () => {
    try {
      const [pubRes, tlsRes] = await Promise.all([fetch('/api/publish'), fetch('/api/publish/tls')])
      if (pubRes.ok) setData(await pubRes.json())
      if (tlsRes.ok) setTls(await tlsRes.json())
      setError(null)
    } catch (err) {
      setError('Failed to load: ' + err.message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchAll()
    const i = setInterval(fetchAll, 5000)
    return () => clearInterval(i)
  }, [fetchAll])

  const flash = (m) => { setMessage(m); setTimeout(() => setMessage(null), 3000) }

  const handleUnpublish = async (haulId) => {
    setError(null)
    try {
      const res = await fetch(`/api/publish/${haulId}`, { method: 'DELETE' })
      if (!res.ok) throw new Error((await res.text()) || 'Unpublish failed')
      await fetchAll()
    } catch (err) { setError(err.message) }
  }

  const handleLoadCert = async (e) => {
    e.preventDefault()
    setSavingTls(true); setError(null)
    try {
      const res = await fetch('/api/publish/tls', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ certPem, keyPem }),
      })
      const body = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(body.error || body.message || 'Failed to load certificate')
      flash(body.message || 'Certificate loaded')
      setCertPem(''); setKeyPem('')
      await fetchAll()
    } catch (err) { setError(err.message) } finally { setSavingTls(false) }
  }

  const handleRevertCert = async () => {
    setError(null)
    try {
      const res = await fetch('/api/publish/tls', { method: 'DELETE' })
      if (!res.ok) throw new Error('Failed to revert certificate')
      flash('Reverted to self-signed certificate')
      await fetchAll()
    } catch (err) { setError(err.message) }
  }

  const provided = tls?.source === 'provided'
  const regPort = data?.registryPort || 5000

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Publishing</h1>
          <p className="page-subtitle">Hauls exposed through the single host-routed registry + file front door</p>
        </div>
        <button className="btn" onClick={() => { setLoading(true); fetchAll() }}>
          <RefreshCw size={16} className={loading ? 'spin' : ''} />
        </button>
      </div>

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

      {/* Registry endpoint summary */}
      <div className="card">
        <div className="card-title">Registry Endpoint</div>
        <table className="data-table">
          <tbody>
            <tr>
              <td style={{ width: '180px' }}>Port (shared)</td>
              <td className="primary"><code>{regPort}</code></td>
              <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>HAULER_UI_REGISTRY_PORT</td>
            </tr>
            <tr>
              <td>Base domain</td>
              <td className="primary"><code>{data?.registryDomain || '(none — using bare slugs)'}</code></td>
              <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>HAULER_UI_REGISTRY_DOMAIN</td>
            </tr>
          </tbody>
        </table>
        <p style={{ fontSize: '0.78rem', color: 'var(--text-muted)', marginTop: '0.6rem', marginBottom: 0 }}>
          All published hauls share this one port; requests route to the right haul by <strong>Host header</strong>
          {' '}(container registries must live at the host root, so routing is host-based, not path-based).
        </p>
      </div>

      {/* Routes table */}
      <div className="card">
        <div className="card-title">Published Hauls</div>
        {loading && !data ? (
          <div className="loading">Loading…</div>
        ) : !data?.routes?.length ? (
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
            No hauls published. Open a haul&apos;s <strong>Serve</strong> tab and click <strong>Publish</strong>.
          </div>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Haul</th>
                <th>Registry Host</th>
                <th>Internal</th>
                <th>Files</th>
                <th style={{ width: '120px' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {data.routes.map((r) => (
                <tr key={r.haulId}>
                  <td className="primary">{r.name}</td>
                  <td><code>{r.hostname}{regPort === 443 ? '' : `:${regPort}`}</code></td>
                  <td style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>127.0.0.1:{r.port}</td>
                  <td><NavLink to={`/hauls/${r.haulId}`} style={{ color: 'var(--accent-amber)' }}>/h/{r.slug}/</NavLink></td>
                  <td>
                    <button className="btn btn-sm" onClick={() => handleUnpublish(r.haulId)} style={{ color: 'var(--accent-red)' }}>
                      <Trash2 size={14} style={{ marginRight: '0.2rem' }} /> Unpublish
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* TLS management */}
      <div className="card">
        <div className="card-title" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          {provided
            ? <ShieldCheck size={18} style={{ color: 'var(--accent-green)' }} />
            : <ShieldAlert size={18} style={{ color: 'var(--accent-amber)' }} />}
          Registry TLS
        </div>
        {tls && (
          <table className="data-table" style={{ marginBottom: '1rem' }}>
            <tbody>
              <tr>
                <td style={{ width: '160px' }}>Certificate</td>
                <td className="primary">
                  {provided
                    ? <span style={{ color: 'var(--accent-green)' }}>Provided (valid for trusting clients)</span>
                    : <span style={{ color: 'var(--accent-amber)' }}>Self-signed (clients must trust or use insecure)</span>}
                </td>
              </tr>
              <tr><td>Subject</td><td><code>{tls.subject || '-'}</code></td></tr>
              <tr><td>SANs</td><td><code>{(tls.dnsNames || []).join(', ') || '-'}</code></td></tr>
              <tr><td>Expires</td><td>{tls.notAfter ? new Date(tls.notAfter).toLocaleString() : '-'}</td></tr>
            </tbody>
          </table>
        )}

        <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
          Load a CA-signed certificate (ideally a wildcard <code>*.{data?.registryDomain || 'your-domain'}</code>) so
          clusters can pull over valid TLS without complaints. Takes effect immediately — no restart.
        </p>
        <form onSubmit={handleLoadCert}>
          <div className="form-group">
            <label className="form-label">Certificate (PEM)</label>
            <textarea
              className="form-input"
              style={{ minHeight: '120px', fontFamily: 'var(--font-mono)', fontSize: '0.78rem' }}
              placeholder="-----BEGIN CERTIFICATE-----"
              value={certPem}
              onChange={(e) => setCertPem(e.target.value)}
              spellCheck="false"
            />
          </div>
          <div className="form-group">
            <label className="form-label">Private Key (PEM)</label>
            <textarea
              className="form-input"
              style={{ minHeight: '120px', fontFamily: 'var(--font-mono)', fontSize: '0.78rem' }}
              placeholder="-----BEGIN PRIVATE KEY-----"
              value={keyPem}
              onChange={(e) => setKeyPem(e.target.value)}
              spellCheck="false"
            />
          </div>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button type="submit" className="btn btn-primary" disabled={savingTls || !certPem || !keyPem}>
              <Globe size={15} style={{ marginRight: '0.3rem' }} />
              {savingTls ? 'Loading…' : 'Load Certificate'}
            </button>
            {provided && (
              <button type="button" className="btn" onClick={handleRevertCert}>
                Revert to self-signed
              </button>
            )}
          </div>
        </form>
      </div>
    </div>
  )
}

export default Publishing
