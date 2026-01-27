import { useState } from 'react'
import { useNavigate, NavLink } from 'react-router-dom'
import { useJobs } from '../App.jsx'

function StoreRemove() {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()

  // Store remove options
  const [match, setMatch] = useState('')
  const [force, setForce] = useState(false)

  const [error, setError] = useState(null)
  const [submitting, setSubmitting] = useState(false)
  const [showConfirmation, setShowConfirmation] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError(null)

    // If not using force, show confirmation dialog first
    if (!force && !showConfirmation) {
      setShowConfirmation(true)
      return
    }

    setSubmitting(true)
    setShowConfirmation(false)

    try {
      if (!match.trim()) {
        setError('Match string is required')
        setSubmitting(false)
        return
      }

      const requestPayload = {
        match: match,
        force: force || undefined,
      }

      // Filter out undefined values
      Object.keys(requestPayload).forEach(key => {
        if (requestPayload[key] === undefined) {
          delete requestPayload[key]
        }
      })

      const res = await fetch('/api/store/remove', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestPayload)
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Remove request failed')
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

  const handleCancelConfirmation = () => {
    setShowConfirmation(false)
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Store Remove</h1>
          <p className="page-subtitle">Remove artifacts from the store using string matching</p>
        </div>
        <NavLink to="/store" className="btn">Back to Store</NavLink>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {/* Experimental Warning */}
      <div className="card" style={{ borderColor: 'var(--accent-amber)', marginBottom: '1rem' }}>
        <div className="card-title" style={{ color: 'var(--accent-amber)' }}>
          Experimental Feature
        </div>
        <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
          <p style={{ marginBottom: '0.5rem' }}>
            <strong>Warning:</strong> The <code>store remove</code> command is experimental.
            The behavior and CLI arguments may change in future versions of hauler.
          </p>
          <p style={{ marginBottom: 0 }}>
            Use with caution and always verify the match string before removing artifacts.
          </p>
        </div>
      </div>

      {/* Confirmation Dialog */}
      {showConfirmation && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>
            Confirm Removal
          </div>
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
            <p style={{ marginBottom: '0.75rem' }}>
              You are about to remove all artifacts matching:
            </p>
            <p style={{ marginBottom: '0.75rem' }}>
              <code style={{ padding: '0.25rem 0.5rem', backgroundColor: 'var(--bg-secondary)', fontSize: '1rem' }}>
                {match}
              </code>
            </p>
            <p style={{ marginBottom: '1rem' }}>
              <strong>This action cannot be undone.</strong> Are you sure you want to proceed?
            </p>
            <div style={{ display: 'flex', gap: '0.75rem' }}>
              <button
                className="btn btn-primary"
                onClick={handleSubmit}
                style={{ backgroundColor: 'var(--accent-red)', borderColor: 'var(--accent-red)' }}
              >
                Yes, Remove Artifacts
              </button>
              <button
                className="btn"
                onClick={handleCancelConfirmation}
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 320px', gap: '1.5rem' }}>
        {/* Main Form */}
        <div>
          <form onSubmit={handleSubmit}>
            {/* Remove Options */}
            <div className="card" style={{ marginBottom: '1rem' }}>
              <div className="card-title">Remove Options</div>

              {/* Match */}
              <div className="form-group">
                <label className="form-label">Match String</label>
                <input
                  className="form-input"
                  placeholder=":latest or busybox or docker.io/library/nginx"
                  value={match}
                  onChange={(e) => setMatch(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  String pattern to match artifacts for removal. Supports partial matching
                  (e.g., <code>:latest</code> for all latest tags, <code>busybox</code> for any image containing &quot;busybox&quot;).
                </div>
              </div>

              {/* Force */}
              <div className="form-group">
                <label className="form-label" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <input
                    type="checkbox"
                    checked={force}
                    onChange={(e) => setForce(e.target.checked)}
                    disabled={submitting}
                    style={{ width: 'auto' }}
                  />
                  <span>Force (--force)</span>
                </label>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Bypass the confirmation prompt. Use with caution as artifacts will be removed immediately.
                </div>
              </div>
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting || !match.trim() || showConfirmation}
              style={{ fontSize: '1rem', padding: '0.75rem 1.5rem' }}
            >
              {submitting ? 'Starting Remove...' : 'Remove Artifacts'}
            </button>
          </form>
        </div>

        {/* Help Panel */}
        <div>
          <div className="card help-panel">
            <div className="card-title">About Store Remove</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                <strong>Store Remove</strong> removes artifacts from the content store
                using simple string matching.
              </p>
              <p>
                This command is <strong style={{ color: 'var(--accent-amber)' }}>experimental</strong> and
                the behavior may change in future versions.
              </p>
              <p style={{ marginBottom: 0 }}>
                The match string performs partial matching, so use specific patterns to avoid
                unintentionally removing multiple artifacts.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Match Examples</div>
            <div style={{ fontSize: '0.8rem', lineHeight: '1.7' }}>
              <p style={{ marginBottom: '0.5rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>By tag:</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: '0.75rem' }}>
                <code>:latest</code> — removes all images tagged latest
              </p>

              <p style={{ marginBottom: '0.5rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>By name:</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: '0.75rem' }}>
                <code>busybox</code> — removes any image containing &quot;busybox&quot;<br/>
                <code>nginx</code> — removes any image containing &quot;nginx&quot;
              </p>

              <p style={{ marginBottom: '0.5rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>Full reference:</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: 0 }}>
                <code>docker.io/library/nginx:latest</code>
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Quick Links</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              <NavLink to="/store" className="btn btn-sm" style={{ textAlign: 'center' }}>
                View Store
              </NavLink>
              <NavLink to="/store/add" className="btn btn-sm" style={{ textAlign: 'center' }}>
                Add Image
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

export default StoreRemove
