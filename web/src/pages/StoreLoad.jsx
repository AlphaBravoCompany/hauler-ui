import { useState } from 'react'
import { useNavigate, NavLink } from 'react-router-dom'
import { useJobs } from '../App.jsx'
import { AlertTriangle, X, Download, Check, AlertCircle } from 'lucide-react'

function StoreLoad() {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()

  // Multiple filenames list (for -f flag)
  const [fileList, setFileList] = useState(['haul.tar.zst'])

  // Clear store option
  const [clearStore, setClearStore] = useState(false)

  const [error, setError] = useState(null)
  const [submitting, setSubmitting] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)

  const handleAddFile = () => {
    setFileList([...fileList, ''])
  }

  const handleRemoveFile = (index) => {
    const newFiles = fileList.filter((_, i) => i !== index)
    if (newFiles.length === 0) {
      setFileList([''])
    } else {
      setFileList(newFiles)
    }
  }

  const handleFileChange = (index, value) => {
    const newFiles = [...fileList]
    newFiles[index] = value
    setFileList(newFiles)
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError(null)

    // Show confirmation if clearing store
    if (clearStore) {
      setShowConfirm(true)
      return
    }

    await doLoad()
  }

  const doLoad = async () => {
    setShowConfirm(false)
    setSubmitting(true)

    try {
      // Use non-empty file paths from list
      const validFiles = fileList.filter(f => f.trim() !== '')
      if (validFiles.length === 0) {
        setError('Please provide at least one file path')
        setSubmitting(false)
        return
      }

      const requestPayload = {
        filenames: validFiles,
        clear: clearStore
      }

      const res = await fetch('/api/store/load', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestPayload)
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Load request failed')
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
          <h1 className="page-title">Store Load</h1>
          <p className="page-subtitle">Load archives into the content store</p>
        </div>
        <NavLink to="/store" className="btn">Back to Store</NavLink>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {/* Warning banner for Docker/Podman tarballs */}
      <div className="card" style={{ borderColor: 'var(--accent-amber)', marginBottom: '1rem' }}>
        <div className="card-title" style={{ color: 'var(--accent-amber)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <AlertTriangle size={18} />
          Archive Format Support
        </div>
        <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
          <p style={{ marginBottom: '0.5rem' }}>
            <strong>Docker-saved tarballs:</strong> Supported as of hauler v1.3. These archives
            created with <code>docker save</code> can be loaded into the store.
          </p>
          <p style={{ marginBottom: 0 }}>
            <strong>Podman tarballs:</strong> Not currently supported. Archives created with
            <code>podman save</code> cannot be loaded at this time.
          </p>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 320px', gap: '1.5rem' }}>
        {/* Main Form */}
        <div>
          <form onSubmit={handleSubmit}>
            {/* File Options */}
            <div className="card" style={{ marginBottom: '1rem' }}>
              <div className="card-title">Archive Files</div>

              <div className="form-group">
                <label className="form-label">Archive File Paths (-f/--filename)</label>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: '0.5rem' }}>
                  Specify one or more archive files to load. Defaults to <code>haul.tar.zst</code>.
                </div>
                {fileList.map((file, index) => (
                  <div key={index} style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem' }}>
                    <input
                      className="form-input"
                      placeholder="haul.tar.zst"
                      value={file}
                      onChange={(e) => handleFileChange(index, e.target.value)}
                      disabled={submitting}
                      style={{ flex: 1 }}
                    />
                    {fileList.length > 1 && (
                      <button
                        type="button"
                        className="btn btn-sm"
                        onClick={() => handleRemoveFile(index)}
                        disabled={submitting}
                        style={{ color: 'var(--accent-red)' }}
                      >
                        <X size={16} />
                      </button>
                    )}
                  </div>
                ))}
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={handleAddFile}
                  disabled={submitting}
                >
                  + Add Another File
                </button>
              </div>

              {/* Clear Store Option */}
              <div className="form-group" style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid var(--border-color)' }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                  <input
                    type="checkbox"
                    checked={clearStore}
                    onChange={(e) => setClearStore(e.target.checked)}
                    disabled={submitting}
                    style={{ cursor: 'pointer' }}
                  />
                  <span style={{ fontWeight: 500 }}>Clear store before loading</span>
                </label>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.25rem', marginLeft: '1.5rem' }}>
                  Remove all existing content from the store before loading archives. Useful for loading a single haul.
                </div>
              </div>
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting || fileList.filter(f => f.trim()).length === 0}
              style={{ fontSize: '1rem', padding: '0.75rem 1.5rem' }}
            >
              {submitting ? 'Starting Load...' : (
                <span style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <Download size={18} />
                  Load Archives{clearStore ? ' (with Clear)' : ''}
                </span>
              )}
            </button>
          </form>
        </div>

        {/* Help Panel */}
        <div>
          <div className="card help-panel">
            <div className="card-title">About Store Load</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                <strong>Store Load</strong> loads previously created archives into your content store.
              </p>
              <p>
                Use this to restore content from archives created with the
                <strong> Store Save</strong> operation.
              </p>
              <p>
                <strong>Provenance Tracking:</strong> When you load a haul, the system tracks which
                archive each item came from. View this on the <NavLink to="/store/contents" style={{ color: 'var(--accent-amber)' }}>Store Contents</NavLink> page.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Supported Formats</div>
            <div style={{ fontSize: '0.8rem', lineHeight: '1.7' }}>
              <p style={{ marginBottom: '0.5rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <Check size={16} style={{ color: 'var(--accent-green)' }} />
                <strong style={{ color: 'var(--accent-green)' }}>Hauler Archives (.tar.zst)</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: '0.75rem', marginLeft: '1.5rem' }}>
                Archives created with <code>hauler store save</code>
              </p>

              <p style={{ marginBottom: '0.5rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <Check size={16} style={{ color: 'var(--accent-green)' }} />
                <strong style={{ color: 'var(--accent-green)' }}>Docker Tarballs (v1.3+)</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: '0.75rem', marginLeft: '1.5rem' }}>
                Archives created with <code>docker save</code>
              </p>

              <p style={{ marginBottom: '0.5rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <X size={16} style={{ color: 'var(--accent-red)' }} />
                <strong style={{ color: 'var(--accent-red)' }}>Podman Tarballs</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginLeft: '1.5rem' }}>
                Not currently supported
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Quick Links</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              <NavLink to="/store/save" className="btn btn-sm" style={{ textAlign: 'center' }}>
                Save Archive
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

      {/* Confirmation Modal */}
      {showConfirm && (
        <div className="modal-overlay" onClick={() => setShowConfirm(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '1rem' }}>
              <AlertCircle size={24} style={{ color: 'var(--accent-amber)' }} />
              <h2 style={{ margin: 0, fontSize: '1.25rem' }}>Clear Store?</h2>
            </div>
            <p style={{ color: 'var(--text-secondary)', marginBottom: '1.5rem' }}>
              This will <strong>remove all existing content</strong> from the store before loading the archive(s).
              This action cannot be undone.
            </p>
            <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end' }}>
              <button
                className="btn"
                onClick={() => setShowConfirm(false)}
                disabled={submitting}
              >
                Cancel
              </button>
              <button
                className="btn btn-primary"
                onClick={doLoad}
                disabled={submitting}
                style={{ backgroundColor: 'var(--accent-amber)', color: 'var(--bg-primary)' }}
              >
                {submitting ? 'Clearing & Loading...' : 'Clear & Load'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default StoreLoad
