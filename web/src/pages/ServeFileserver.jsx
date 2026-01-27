import { useState, useEffect } from 'react'
import { NavLink } from 'react-router-dom'

function ServeFileserver() {

  // Form state
  const [port, setPort] = useState(8080)
  const [timeout, setTimeout] = useState('')
  const [tlsCert, setTlsCert] = useState('')
  const [tlsKey, setTlsKey] = useState('')
  const [directory, setDirectory] = useState('')

  // UI state
  const [error, setError] = useState(null)
  const [submitting, setSubmitting] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState(false)

  // Processes state
  const [processes, setProcesses] = useState([])
  const [loadingProcesses, setLoadingProcesses] = useState(true)

  // Fetch running processes
  const fetchProcesses = async () => {
    try {
      const res = await fetch('/api/serve/fileserver')
      if (res.ok) {
        const data = await res.json()
        setProcesses(data)
      }
    } catch (err) {
      console.error('Failed to fetch processes:', err)
    } finally {
      setLoadingProcesses(false)
    }
  }

  useEffect(() => {
    fetchProcesses()
    const interval = setInterval(fetchProcesses, 5000)
    return () => clearInterval(interval)
  }, [])

  const handleStart = async (e) => {
    e.preventDefault()
    setError(null)
    setSubmitting(true)

    try {
      const requestPayload = {
        port: port || 8080,
        timeout: timeout ? parseInt(timeout) : undefined,
        tlsCert: tlsCert || undefined,
        tlsKey: tlsKey || undefined,
        directory: directory || undefined,
      }

      // Filter out undefined values
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
        throw new Error(data.message || 'Start request failed')
      }

      // Refresh processes list
      await fetchProcesses()
    } catch (err) {
      setError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  const handleStop = async (pid) => {
    setError(null)
    setSubmitting(true)

    try {
      const res = await fetch(`/api/serve/fileserver/${pid}`, {
        method: 'DELETE'
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Stop request failed')
      }

      // Refresh processes list
      await fetchProcesses()
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
          <h1 className="page-title">Serve Fileserver</h1>
          <p className="page-subtitle">Start and stop the embedded hauler fileserver</p>
        </div>
        <NavLink to="/serve" className="btn">Back to Serve</NavLink>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 320px', gap: '1.5rem' }}>
        {/* Main Form */}
        <div>
          <form onSubmit={handleStart}>
            {/* Basic Options */}
            <div className="card" style={{ marginBottom: '1rem' }}>
              <div className="card-title">Fileserver Options</div>

              {/* Port */}
              <div className="form-group">
                <label className="form-label">Port (--port)</label>
                <input
                  className="form-input"
                  type="number"
                  min="1"
                  max="65535"
                  placeholder="8080"
                  value={port}
                  onChange={(e) => setPort(parseInt(e.target.value) || 8080)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Port for the fileserver to listen on (default: 8080)
                </div>
              </div>

              {/* Timeout */}
              <div className="form-group">
                <label className="form-label">Timeout (--timeout)</label>
                <input
                  className="form-input"
                  type="number"
                  min="0"
                  placeholder="0"
                  value={timeout}
                  onChange={(e) => setTimeout(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Timeout duration in seconds for serving files (default: 0 / no timeout)
                </div>
              </div>

              {/* Advanced Toggle */}
              <button
                type="button"
                className="btn btn-sm"
                onClick={() => setShowAdvanced(!showAdvanced)}
                style={{ marginTop: '0.5rem' }}
              >
                {showAdvanced ? '▼ Hide Advanced' : '▶ Show Advanced'}
              </button>

              {/* Advanced Options */}
              {showAdvanced && (
                <div style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid var(--border-color)' }}>
                  {/* TLS Cert */}
                  <div className="form-group">
                    <label className="form-label">TLS Certificate Path (--tls-cert)</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/cert.pem"
                      value={tlsCert}
                      onChange={(e) => setTlsCert(e.target.value)}
                      disabled={submitting}
                    />
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Path to TLS certificate file for HTTPS
                    </div>
                  </div>

                  {/* TLS Key */}
                  <div className="form-group">
                    <label className="form-label">TLS Key Path (--tls-key)</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/key.pem"
                      value={tlsKey}
                      onChange={(e) => setTlsKey(e.target.value)}
                      disabled={submitting}
                    />
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Path to TLS private key file for HTTPS
                    </div>
                  </div>

                  {/* Directory */}
                  <div className="form-group">
                    <label className="form-label">Store Directory (--directory)</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/store"
                      value={directory}
                      onChange={(e) => setDirectory(e.target.value)}
                      disabled={submitting}
                    />
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Override the default store directory
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting}
              style={{ fontSize: '1rem', padding: '0.75rem 1.5rem' }}
            >
              {submitting ? 'Starting...' : '🚀 Start Fileserver'}
            </button>
          </form>

          {/* Running Processes */}
          <div className="card" style={{ marginTop: '1.5rem' }}>
            <div className="card-title">Fileserver Processes</div>
            {loadingProcesses ? (
              <div style={{ color: 'var(--text-secondary)' }}>Loading...</div>
            ) : processes.length === 0 ? (
              <div style={{ color: 'var(--text-secondary)' }}>No fileserver processes running</div>
            ) : (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>PID</th>
                    <th>Port</th>
                    <th>Status</th>
                    <th>Started</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {processes.map(proc => (
                    <tr key={proc.id}>
                      <td className="primary">#{proc.pid}</td>
                      <td>{proc.port}</td>
                      <td><span className={`badge ${getStatusBadgeClass(proc.status)}`}>{proc.status}</span></td>
                      <td style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                        {formatTime(proc.startedAt)}
                      </td>
                      <td>
                        {proc.status === 'running' && (
                          <button
                            className="btn btn-sm"
                            onClick={() => handleStop(proc.pid)}
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
        </div>

        {/* Help Panel */}
        <div>
          <div className="card help-panel">
            <div className="card-title">About Serve Fileserver</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                <strong>Serve Fileserver</strong> starts an embedded HTTP file server
                that serves content from your hauler store.
              </p>
              <p>
                This is useful for accessing stored files (charts, files, etc.) via HTTP
                in air-gapped environments or for local testing.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Accessing the Fileserver</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginBottom: '0.5rem' }}>
                Once running, access the fileserver at:
              </p>
              <code style={{ display: 'block', padding: '0.5rem', backgroundColor: 'var(--bg-primary)', borderRadius: '4px' }}>
                localhost:{port}
              </code>
              <p style={{ marginTop: '0.75rem', marginBottom: '0.5rem' }}>
                To download files:
              </p>
              <code style={{ display: 'block', padding: '0.5rem', backgroundColor: 'var(--bg-primary)', borderRadius: '4px', fontSize: '0.75rem' }}>
                curl http://localhost:{port}/&lt;file-path&gt;
              </code>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Quick Links</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              <NavLink to="/store" className="btn btn-sm" style={{ textAlign: 'center' }}>
                View Store
              </NavLink>
              <NavLink to="/serve/registry" className="btn btn-sm" style={{ textAlign: 'center' }}>
                Serve Registry
              </NavLink>
              <NavLink to="/jobs" className="btn btn-sm" style={{ textAlign: 'center' }}>
                Job History
              </NavLink>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default ServeFileserver
