import { useState, useEffect } from 'react'
import { useNavigate, NavLink } from 'react-router-dom'
import { useJobs } from '../App.jsx'

function StoreExtract() {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()

  // Extract options from hauler store extract --help
  const [artifactRef, setArtifactRef] = useState('')
  const [outputDir, setOutputDir] = useState('')

  // Job result state for showing extraction results after completion
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
        artifactRef: artifactRef || undefined,
        outputDir: outputDir || undefined,
      }

      // Filter out undefined values
      Object.keys(requestPayload).forEach(key => {
        if (requestPayload[key] === undefined) {
          delete requestPayload[key]
        }
      })

      const res = await fetch('/api/store/extract', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestPayload)
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Extract request failed')
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
          <h1 className="page-title">Store Extract</h1>
          <p className="page-subtitle">Pull specific artifacts from the store to disk</p>
        </div>
        <NavLink to="/store" className="btn">Back to Store</NavLink>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {/* Success message shown after successful job completion */}
      {jobResult && jobResult.outputDir && (
        <div className="card" style={{ borderColor: 'var(--accent-green)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-green)' }}>
            ✓ Extraction Complete
          </div>
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
            <p style={{ marginBottom: '0.5rem' }}>
              Your artifact has been extracted successfully.
            </p>
            <p style={{ marginBottom: '0.75rem' }}>
              <strong>Output directory:</strong> <code>{jobResult.outputDir}</code>
            </p>
          </div>
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 320px', gap: '1.5rem' }}>
        {/* Main Form */}
        <div>
          <form onSubmit={handleSubmit}>
            {/* Extract Options */}
            <div className="card" style={{ marginBottom: '1rem' }}>
              <div className="card-title">Extract Options</div>

              {/* Artifact Reference */}
              <div className="form-group">
                <label className="form-label">Artifact Reference</label>
                <input
                  className="form-input"
                  placeholder="docker.io/library/nginx:latest"
                  value={artifactRef}
                  onChange={(e) => setArtifactRef(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  The artifact reference to extract from the store (e.g., docker.io/library/nginx:latest).
                </div>
              </div>

              {/* Output Directory */}
              <div className="form-group">
                <label className="form-label">Output Directory (--output)</label>
                <input
                  className="form-input"
                  placeholder="./output (optional)"
                  value={outputDir}
                  onChange={(e) => setOutputDir(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Directory where extracted files will be saved. Defaults to current directory if not specified.
                </div>
              </div>
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting || !artifactRef.trim()}
              style={{ fontSize: '1rem', padding: '0.75rem 1.5rem' }}
            >
              {submitting ? 'Starting Extract...' : '📤 Extract Artifact'}
            </button>
          </form>
        </div>

        {/* Help Panel */}
        <div>
          <div className="card help-panel">
            <div className="card-title">About Store Extract</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                <strong>Store Extract</strong> pulls specific artifacts from your content store
                and extracts them to disk.
              </p>
              <p>
                This is useful for retrieving individual images, charts, or files that were
                previously added to the store.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Artifact References</div>
            <div style={{ fontSize: '0.8rem', lineHeight: '1.7' }}>
              <p style={{ marginBottom: '0.5rem' }}>
                Use the same reference format that was used when adding the artifact:
              </p>
              <ul style={{ paddingLeft: '1.25rem', margin: '0.5rem 0', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                <li><strong>Images:</strong> docker.io/library/nginx:latest</li>
                <li><strong>Charts:</strong> repo/chart-name or oci://registry/chart</li>
                <li><strong>Files:</strong> The name or URL used when added</li>
              </ul>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Quick Links</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              <NavLink to="/store/add" className="btn btn-sm" style={{ textAlign: 'center' }}>
                Add Image
              </NavLink>
              <NavLink to="/store/add-chart" className="btn btn-sm" style={{ textAlign: 'center' }}>
                Add Chart
              </NavLink>
              <NavLink to="/store/add-file" className="btn btn-sm" style={{ textAlign: 'center' }}>
                Add File
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

export default StoreExtract
