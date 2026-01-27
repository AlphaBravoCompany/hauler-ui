import { useState } from 'react'
import { useNavigate, NavLink } from 'react-router-dom'
import { useJobs } from '../App.jsx'

function StoreAddFile() {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()

  const [filePath, setFilePath] = useState('')
  const [url, setUrl] = useState('')
  const [name, setName] = useState('')

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError(null)

    // Mutually exclusive validation
    if (!filePath && !url) {
      setError('Either file path or URL is required')
      return
    }

    if (filePath && url) {
      setError('Please provide either a file path or URL, not both')
      return
    }

    setLoading(true)

    try {
      const res = await fetch('/api/store/add-file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          filePath: filePath || undefined,
          url: url || undefined,
          name: name || undefined
        })
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Add file request failed')
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
          <h1 className="page-title">Store Add File</h1>
          <p className="page-subtitle">Add local files or remote URLs to the hauler store</p>
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
            <div className="card-title">File Source</div>
            <form onSubmit={handleSubmit}>
              <div className="form-group">
                <label className="form-label">File Path</label>
                <input
                  className="form-input"
                  placeholder="/path/to/local/file.txt"
                  value={filePath}
                  onChange={(e) => setFilePath(e.target.value)}
                  disabled={loading}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Path to a local file on the filesystem
                </div>
              </div>

              <div style={{
                display: 'flex',
                alignItems: 'center',
                margin: '1rem 0',
                color: 'var(--text-muted)',
                fontSize: '0.85rem'
              }}>
                <div style={{
                  flex: 1,
                  height: '1px',
                  backgroundColor: 'var(--border-color)'
                }}></div>
                <span style={{
                  padding: '0 1rem',
                  fontWeight: '500',
                  color: 'var(--text-secondary)'
                }}>OR</span>
                <div style={{
                  flex: 1,
                  height: '1px',
                  backgroundColor: 'var(--border-color)'
                }}></div>
              </div>

              <div className="form-group">
                <label className="form-label">URL</label>
                <input
                  className="form-input"
                  placeholder="https://example.com/file.txt"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  disabled={loading}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  URL to a remote file to download
                </div>
              </div>

              <div className="form-group">
                <label className="form-label">Name (Optional)</label>
                <input
                  className="form-input"
                  placeholder="custom-filename.txt"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  disabled={loading}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Rewrite the name of the file when storing (defaults to original filename)
                </div>
              </div>

              <button
                type="submit"
                className="btn btn-primary"
                disabled={loading || (!filePath && !url)}
              >
                {loading ? 'Adding File...' : 'Add File to Store'}
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
                <strong style={{ color: 'var(--accent-amber)' }}>Local file:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                /path/to/file.txt
              </code>

              <p style={{ marginBottom: '0.75rem', marginTop: '1rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>Remote URL:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                https://get.rke2.io/install.sh
              </code>

              <p style={{ marginBottom: '0.75rem', marginTop: '1rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>With custom name:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                https://get.hauler.dev<br />
                Name: hauler-install.sh
              </code>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">About Adding Files</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                Hauler can store <strong>local files</strong> or download files from <strong>remote URLs</strong>.
              </p>
              <p>
                Use the optional <strong>Name</strong> field to rename the file when storing it, useful for giving downloaded files a meaningful name.
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default StoreAddFile
