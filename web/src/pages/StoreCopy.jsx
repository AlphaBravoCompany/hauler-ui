import { useState } from 'react'
import { useNavigate, NavLink } from 'react-router-dom'
import { useJobs } from '../App.jsx'

function StoreCopy() {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()

  // Store copy options
  const [target, setTarget] = useState('')
  const [insecure, setInsecure] = useState(false)
  const [plainHttp, setPlainHttp] = useState(false)
  const [only, setOnly] = useState('')

  const [error, setError] = useState(null)
  const [submitting, setSubmitting] = useState(false)

  // Determine if target is a registry (starts with registry://)
  const isRegistryTarget = target.startsWith('registry://')
  const isDirTarget = target.startsWith('dir://')

  // Show warning if registry target includes a path
  const showRegistryPathWarning = isRegistryTarget && target.replace('registry://', '').includes('/')

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError(null)
    setSubmitting(true)

    try {
      if (!target.trim()) {
        setError('Target is required')
        setSubmitting(false)
        return
      }

      // Validate target format
      if (!target.startsWith('registry://') && !target.startsWith('dir://')) {
        setError('Target must start with registry:// or dir://')
        setSubmitting(false)
        return
      }

      const requestPayload = {
        target: target,
        insecure: insecure || undefined,
        plainHttp: plainHttp || undefined,
        only: only || undefined,
      }

      // Filter out undefined values
      Object.keys(requestPayload).forEach(key => {
        if (requestPayload[key] === undefined) {
          delete requestPayload[key]
        }
      })

      const res = await fetch('/api/store/copy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestPayload)
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Copy request failed')
      }

      // Refresh jobs list and navigate to job detail
      await fetchJobs()
      navigate(`/jobs/${data.jobId}`)
    } catch (err) {
      setError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Store Copy</h1>
          <p className="page-subtitle">Copy store contents to a registry or directory</p>
        </div>
        <NavLink to="/store" className="btn">Back to Store</NavLink>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {/* Warning for registry path usage */}
      {showRegistryPathWarning && (
        <div className="card" style={{ borderColor: 'var(--accent-amber)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-amber)' }}>
            Registry Login Required
          </div>
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
            <p style={{ marginBottom: '0.5rem' }}>
              <strong>Important:</strong> When copying to a registry path, you must first login
              to the <strong>registry root</strong> (without the path).
            </p>
            <p style={{ marginBottom: '0.5rem' }}>
              For example, if copying to <code>registry://docker.io/my-org/my-path</code>,
              you must first login to just <code>docker.io</code> (without the path).
            </p>
            <p style={{ marginBottom: 0 }}>
              <NavLink to="/registry" className="btn btn-sm" style={{ display: 'inline-block' }}>
                Go to Registry Login
              </NavLink>
            </p>
          </div>
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 320px', gap: '1.5rem' }}>
        {/* Main Form */}
        <div>
          <form onSubmit={handleSubmit}>
            {/* Target Options */}
            <div className="card" style={{ marginBottom: '1rem' }}>
              <div className="card-title">Copy Options</div>

              {/* Target */}
              <div className="form-group">
                <label className="form-label">Target</label>
                <input
                  className="form-input"
                  placeholder="registry://docker.io/my-org or dir:///path/to/export"
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Destination for copying store contents. Use <code>registry://</code> for a container registry
                  or <code>dir://</code> for a directory export.
                </div>
              </div>

              {/* Registry-specific options */}
              {isRegistryTarget && (
                <>
                  {/* Insecure */}
                  <div className="form-group">
                    <label className="form-label" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                      <input
                        type="checkbox"
                        checked={insecure}
                        onChange={(e) => setInsecure(e.target.checked)}
                        disabled={submitting}
                        style={{ width: 'auto' }}
                      />
                      <span>Insecure (--insecure)</span>
                    </label>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Allow connections to registries using self-signed certificates or without TLS.
                    </div>
                  </div>

                  {/* Plain HTTP */}
                  <div className="form-group">
                    <label className="form-label" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                      <input
                        type="checkbox"
                        checked={plainHttp}
                        onChange={(e) => setPlainHttp(e.target.checked)}
                        disabled={submitting}
                        style={{ width: 'auto' }}
                      />
                      <span>Plain HTTP (--plain-http)</span>
                    </label>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Use plain HTTP (non-HTTPS) for registry connections.
                    </div>
                  </div>

                  {/* Only */}
                  <div className="form-group">
                    <label className="form-label">Only (--only)</label>
                    <select
                      className="form-input"
                      value={only}
                      onChange={(e) => setOnly(e.target.value)}
                      disabled={submitting}
                    >
                      <option value="">Copy all content (signatures + atts)</option>
                      <option value="sig">Signatures only</option>
                      <option value="att">Attestations only</option>
                    </select>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Filter content to copy: only signatures, only attestations, or all.
                    </div>
                  </div>
                </>
              )}

              {/* Directory-specific hint */}
              {isDirTarget && (
                <div className="form-group">
                  <div style={{
                    padding: '0.75rem',
                    backgroundColor: 'var(--bg-secondary)',
                    border: '1px solid var(--border-color)',
                    borderRadius: '4px',
                    fontSize: '0.85rem',
                    color: 'var(--text-secondary)'
                  }}>
                    Directory export will copy all store contents to the specified path.
                    The directory must exist or be creatable by the hauler process.
                  </div>
                </div>
              )}
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting || !target.trim()}
              style={{ fontSize: '1rem', padding: '0.75rem 1.5rem' }}
            >
              {submitting ? 'Starting Copy...' : '📋 Copy Store'}
            </button>
          </form>
        </div>

        {/* Help Panel */}
        <div>
          <div className="card help-panel">
            <div className="card-title">About Store Copy</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                <strong>Store Copy</strong> copies your entire content store to another registry
                or exports it to a directory.
              </p>
              <p>
                This is useful for mirroring content to an air-gapped registry or for
                exporting store contents for manual inspection.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Target Formats</div>
            <div style={{ fontSize: '0.8rem', lineHeight: '1.7' }}>
              <p style={{ marginBottom: '0.5rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>Registry:</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: '0.75rem' }}>
                <code>registry://docker.io/my-org/my-path</code><br/>
                <code>registry://ghcr.io/username/repo</code>
              </p>

              <p style={{ marginBottom: '0.5rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>Directory:</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: '0.75rem' }}>
                <code>dir:///data/export</code><br/>
                <code>dir://./local-export</code>
              </p>

              <p style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.5rem', marginBottom: 0 }}>
                For directory targets, the path must be accessible to the hauler process.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Quick Links</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              <NavLink to="/registry" className="btn btn-sm" style={{ textAlign: 'center' }}>
                Registry Login
              </NavLink>
              <NavLink to="/store" className="btn btn-sm" style={{ textAlign: 'center' }}>
                View Store
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

export default StoreCopy
