import { useState, useEffect } from 'react'
import { useNavigate, NavLink } from 'react-router-dom'
import { useJobs } from '../App.jsx'
import { Check, Save } from 'lucide-react'

function StoreSave() {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()

  // Save options from hauler store save --help
  const [filename, setFilename] = useState('haul.tar.zst')
  const [platform, setPlatform] = useState('')
  const [containerd, setContainerd] = useState('')

  // Job result state for showing download link after completion
  const [lastJobId, setLastJobId] = useState(null)
  const [jobResult, setJobResult] = useState(null)
  const [error, setError] = useState(null)

  const [submitting, setSubmitting] = useState(false)

  // Poll for job result if we have a lastJobId
  useEffect(() => {
    if (!lastJobId) return

    const pollInterval = setInterval(async () => {
      try {
        const res = await fetch(`/api/jobs/${lastJobId}`)
        if (res.ok) {
          const job = await res.json()
          if (job.status === 'succeeded' && job.result) {
            try {
              const result = JSON.parse(job.result)
              setJobResult(result)
              clearInterval(pollInterval)
            } catch (e) {
              console.error('Failed to parse job result:', e)
            }
          } else if (job.status === 'failed') {
            clearInterval(pollInterval)
          }
        }
      } catch (err) {
        console.error('Error polling job status:', err)
      }
    }, 1000)

    return () => clearInterval(pollInterval)
  }, [lastJobId])

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError(null)
    setJobResult(null)
    setSubmitting(true)

    try {
      const requestPayload = {
        filename: filename || undefined,
        platform: platform || undefined,
        containerd: containerd || undefined,
      }

      // Filter out undefined values
      Object.keys(requestPayload).forEach(key => {
        if (requestPayload[key] === undefined) {
          delete requestPayload[key]
        }
      })

      const res = await fetch('/api/store/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestPayload)
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Save request failed')
      }

      setLastJobId(data.jobId)

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
          <h1 className="page-title">Store Save</h1>
          <p className="page-subtitle">Package your store into a portable archive</p>
        </div>
        <NavLink to="/store" className="btn">Back to Store</NavLink>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {/* Download link shown after successful job completion */}
      {jobResult && jobResult.archivePath && (
        <div className="card" style={{ borderColor: 'var(--accent-green)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-green)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Check size={20} />
            Archive Ready
          </div>
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
            <p style={{ marginBottom: '0.5rem' }}>
              Your store has been packaged successfully.
            </p>
            <p style={{ marginBottom: '0.75rem' }}>
              <strong>Archive path:</strong> <code>{jobResult.archivePath}</code>
            </p>
            <a
              href={`/api/downloads/${jobResult.filename}`}
              className="btn btn-primary"
              download
            >
              Download {jobResult.filename}
            </a>
          </div>
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 320px', gap: '1.5rem' }}>
        {/* Main Form */}
        <div>
          <form onSubmit={handleSubmit}>
            {/* Save Options */}
            <div className="card" style={{ marginBottom: '1rem' }}>
              <div className="card-title">Save Options</div>

              {/* Filename */}
              <div className="form-group">
                <label className="form-label">Output Filename (--filename)</label>
                <input
                  className="form-input"
                  placeholder="haul.tar.zst"
                  value={filename}
                  onChange={(e) => setFilename(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Name of the archive file to create. Will be saved in the data directory.
                </div>
              </div>

              {/* Platform */}
              <div className="form-group">
                <label className="form-label">Platform (-p)</label>
                <input
                  className="form-input"
                  placeholder="linux/amd64 (optional)"
                  value={platform}
                  onChange={(e) => setPlatform(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Specify the platform for the archive contents (e.g., linux/amd64, linux/arm64).
                </div>
              </div>

              {/* Containerd Target */}
              <div className="form-group">
                <label className="form-label">Containerd Target (--containerd)</label>
                <input
                  className="form-input"
                  placeholder="namespace:context (optional)"
                  value={containerd}
                  onChange={(e) => setContainerd(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Optional containerd namespace:context for importing directly to containerd.
                </div>
              </div>
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting || !filename.trim()}
              style={{ fontSize: '1rem', padding: '0.75rem 1.5rem' }}
            >
              {submitting ? 'Starting Save...' : (
                <span style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <Save size={18} />
                  Save Store
                </span>
              )}
            </button>
          </form>
        </div>

        {/* Help Panel */}
        <div>
          <div className="card help-panel">
            <div className="card-title">About Store Save</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                <strong>Store Save</strong> packages your entire content store into a portable
                compressed archive file.
              </p>
              <p>
                The archive can be transferred to air-gapped systems and loaded using
                the <strong>Store Load</strong> operation.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Archive Format</div>
            <div style={{ fontSize: '0.8rem', lineHeight: '1.7' }}>
              <p style={{ marginBottom: '0.5rem' }}>
                Archives are created in <strong>.tar.zst</strong> format (Zstandard compressed).
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: '0.75rem' }}>
                This provides excellent compression ratios while maintaining fast decompression speeds.
              </p>

              <p style={{ marginBottom: '0.5rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>After saving:</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0 }}>
                Once the save operation completes, you can download the archive directly from this page.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Quick Links</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              <NavLink to="/store/load" className="btn btn-sm" style={{ textAlign: 'center' }}>
                Load Archive
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

export default StoreSave
