import { useState, useEffect } from 'react'
import { useNavigate, NavLink } from 'react-router-dom'
import { useJobs } from '../App.jsx'

function StoreAddChart() {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()

  const [name, setName] = useState('')
  const [repoUrl, setRepoUrl] = useState('')
  const [version, setVersion] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [keyFile, setKeyFile] = useState('')
  const [certFile, setCertFile] = useState('')
  const [caFile, setCaFile] = useState('')
  const [insecureSkipTlsVerify, setInsecureSkipTlsVerify] = useState(false)
  const [plainHttp, setPlainHttp] = useState(false)
  const [verify, setVerify] = useState(false)
  const [addDependencies, setAddDependencies] = useState(false)
  const [addImages, setAddImages] = useState(false)

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [showAuth, setShowAuth] = useState(false)

  // Fetch capabilities to check for --add-dependencies and --add-images support
  const [capabilities, setCapabilities] = useState(null)

  useEffect(() => {
    fetch('/api/hauler/capabilities')
      .then(res => res.json())
      .then(data => setCapabilities(data))
      .catch(() => setCapabilities(null))
  }, [])

  // Check if a flag is supported by looking at subcommands
  const hasFlagSupport = (flagName) => {
    if (!capabilities || !capabilities.subcommands) return false

    // Find the 'store' subcommand
    const storeCmd = capabilities.subcommands.find(sc => sc.Name === 'store')
    if (!storeCmd) return false

    // Check if the flag exists in store's flags
    return storeCmd.Flags.some(f => f.Name.startsWith(flagName))
  }

  const supportsAddDependencies = hasFlagSupport('add-dependencies')
  const supportsAddImages = hasFlagSupport('add-images')

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError(null)
    setLoading(true)

    try {
      const res = await fetch('/api/store/add-chart', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name,
          repoUrl: repoUrl || undefined,
          version: version || undefined,
          username: username || undefined,
          password: password || undefined,
          keyFile: keyFile || undefined,
          certFile: certFile || undefined,
          caFile: caFile || undefined,
          insecureSkipTlsVerify,
          plainHttp,
          verify,
          addDependencies,
          addImages
        })
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Add chart request failed')
      }

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
          <h1 className="page-title">Store Add Chart</h1>
          <p className="page-subtitle">Add Helm charts to the hauler store</p>
        </div>
        <NavLink to="/store" className="btn">Back to Store</NavLink>
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
          <div className="card">
            <div className="card-title">Chart Information</div>
            <form onSubmit={handleSubmit}>
              <div className="form-group">
                <label className="form-label">Chart Name/Reference *</label>
                <input
                  className="form-input"
                  placeholder="nginx-stable/nginx or oci://ghcr.io/mychart"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  disabled={loading}
                  required
                  autoFocus
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  The chart reference to store (e.g., repo/name, or full OCI path)
                </div>
              </div>

              <div className="form-group">
                <label className="form-label">Repository URL (Optional)</label>
                <input
                  className="form-input"
                  placeholder="https://charts.helm.sh/stable"
                  value={repoUrl}
                  onChange={(e) => setRepoUrl(e.target.value)}
                  disabled={loading}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Helm chart repository URL (not needed for OCI charts)
                </div>
              </div>

              <div className="form-group">
                <label className="form-label">Version (Optional)</label>
                <input
                  className="form-input"
                  placeholder="1.2.3"
                  value={version}
                  onChange={(e) => setVersion(e.target.value)}
                  disabled={loading}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Specific chart version (defaults to latest)
                </div>
              </div>

              <button
                type="button"
                className="btn btn-sm"
                onClick={() => setShowAuth(!showAuth)}
                style={{ marginBottom: '1rem' }}
              >
                {showAuth ? '▼' : '▶'} {showAuth ? 'Hide' : 'Show'} Authentication Options
              </button>

              {showAuth && (
                <div style={{
                  padding: '1rem',
                  backgroundColor: 'var(--bg-tertiary)',
                  border: '1px dashed var(--border-color)',
                  borderRadius: '2px',
                  marginBottom: '1rem'
                }}>
                  <div className="card-title" style={{ border: 'none', paddingBottom: '0' }}>Authentication</div>

                  <div className="form-group">
                    <label className="form-label">Username (Optional)</label>
                    <input
                      className="form-input"
                      placeholder="username"
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      disabled={loading}
                    />
                  </div>

                  <div className="form-group">
                    <label className="form-label">Password (Optional)</label>
                    <input
                      className="form-input"
                      type="password"
                      placeholder="••••••••"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      disabled={loading}
                    />
                  </div>
                </div>
              )}

              <button
                type="button"
                className="btn btn-sm"
                onClick={() => setShowAdvanced(!showAdvanced)}
                style={{ marginBottom: '1rem' }}
              >
                {showAdvanced ? '▼' : '▶'} {showAdvanced ? 'Hide' : 'Show'} Advanced Options
              </button>

              {showAdvanced && (
                <div style={{
                  padding: '1rem',
                  backgroundColor: 'var(--bg-tertiary)',
                  border: '1px dashed var(--border-color)',
                  borderRadius: '2px',
                  marginBottom: '1rem'
                }}>
                  <div className="card-title" style={{ border: 'none', paddingBottom: '0' }}>TLS & Security</div>

                  <div className="form-group">
                    <label className="form-label">Key File Path (Optional)</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/client.key"
                      value={keyFile}
                      onChange={(e) => setKeyFile(e.target.value)}
                      disabled={loading}
                    />
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Path to TLS client key file for authentication
                    </div>
                  </div>

                  <div className="form-group">
                    <label className="form-label">Certificate File Path (Optional)</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/client.crt"
                      value={certFile}
                      onChange={(e) => setCertFile(e.target.value)}
                      disabled={loading}
                    />
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Path to TLS client certificate file for authentication
                    </div>
                  </div>

                  <div className="form-group">
                    <label className="form-label">CA File Path (Optional)</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/ca.crt"
                      value={caFile}
                      onChange={(e) => setCaFile(e.target.value)}
                      disabled={loading}
                    />
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Path to CA certificate file for TLS verification
                    </div>
                  </div>

                  <div className="form-group">
                    <label className="form-label" style={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}>
                      <input
                        type="checkbox"
                        checked={insecureSkipTlsVerify}
                        onChange={(e) => setInsecureSkipTlsVerify(e.target.checked)}
                        disabled={loading}
                        style={{ marginRight: '0.5rem' }}
                      />
                      Insecure Skip TLS Verify
                    </label>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Skip TLS certificate verification (not recommended for production)
                    </div>
                  </div>

                  <div className="form-group">
                    <label className="form-label" style={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}>
                      <input
                        type="checkbox"
                        checked={plainHttp}
                        onChange={(e) => setPlainHttp(e.target.checked)}
                        disabled={loading}
                        style={{ marginRight: '0.5rem' }}
                      />
                      Plain HTTP
                    </label>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Use plain HTTP instead of HTTPS for the repository
                    </div>
                  </div>

                  <div className="form-group">
                    <label className="form-label" style={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}>
                      <input
                        type="checkbox"
                        checked={verify}
                        onChange={(e) => setVerify(e.target.checked)}
                        disabled={loading}
                        style={{ marginRight: '0.5rem' }}
                      />
                      Verify
                    </label>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Verify the chart before storing
                    </div>
                  </div>

                  {(supportsAddDependencies || supportsAddImages) && (
                    <div className="card-title" style={{ border: 'none', paddingTop: '1rem', paddingBottom: '0' }}>Chart Options</div>
                  )}

                  {supportsAddDependencies && (
                    <div className="form-group">
                      <label className="form-label" style={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}>
                        <input
                          type="checkbox"
                          checked={addDependencies}
                          onChange={(e) => setAddDependencies(e.target.checked)}
                          disabled={loading}
                          style={{ marginRight: '0.5rem' }}
                        />
                        Add Dependencies
                      </label>
                      <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                        Download and store chart dependencies
                      </div>
                    </div>
                  )}

                  {supportsAddImages && (
                    <div className="form-group">
                      <label className="form-label" style={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}>
                        <input
                          type="checkbox"
                          checked={addImages}
                          onChange={(e) => setAddImages(e.target.checked)}
                          disabled={loading}
                          style={{ marginRight: '0.5rem' }}
                        />
                        Add Images
                      </label>
                      <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                        Download and store container images referenced by the chart
                      </div>
                    </div>
                  )}
                </div>
              )}

              <button type="submit" className="btn btn-primary" disabled={loading || !name}>
                {loading ? 'Adding Chart...' : 'Add Chart to Store'}
              </button>
            </form>
          </div>
        </div>

        {/* Help Panel */}
        <div>
          <div className="card help-panel">
            <div className="card-title">Examples</div>
            <div style={{ fontSize: '0.8rem', lineHeight: '1.7' }}>
              <p style={{ marginBottom: '0.75rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>Simple Helm chart:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                nginx-stable/nginx
              </code>

              <p style={{ marginBottom: '0.75rem', marginTop: '1rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>With specific version:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                nginx-stable/nginx<br />
                Version: 1.2.3
              </code>

              <p style={{ marginBottom: '0.75rem', marginTop: '1rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>OCI-based chart:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                oci://ghcr.io/myorg/mychart
              </code>

              <p style={{ marginBottom: '0.75rem', marginTop: '1rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>Custom repository:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                mychart<br />
                Repo URL: https://charts.example.com
              </code>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">About Helm Charts</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                Hauler supports storing <strong>Helm charts</strong> from both HTTP repositories and OCI registries.
              </p>
              <p>
                For HTTP repositories, provide the chart name and optionally the repository URL. For OCI-based charts, use the full <code>oci://</code> path.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">TLS & Authentication</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                Private chart repositories may require <strong>authentication</strong> via username/password or client certificates.
              </p>
              <p>
                Use TLS options to configure certificate verification for repositories with custom or self-signed certificates.
              </p>
            </div>
          </div>

          {(supportsAddDependencies || supportsAddImages) && (
            <div className="card help-panel" style={{ marginTop: '1rem' }}>
              <div className="card-title">Capability-Driven Options</div>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
                <p style={{ marginTop: 0 }}>
                  The following options are available because your hauler version supports them:
                </p>
                <ul style={{ paddingLeft: '1rem', margin: '0.5rem 0' }}>
                  {supportsAddDependencies && <li><strong>Add Dependencies:</strong> Store chart dependencies</li>}
                  {supportsAddImages && <li><strong>Add Images:</strong> Store container images from the chart</li>}
                </ul>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default StoreAddChart
