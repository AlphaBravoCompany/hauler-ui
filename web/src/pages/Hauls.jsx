import { useState, useEffect, useRef } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { useJobs } from '../App.jsx'
import { Package, RefreshCw, Download, Upload, Trash2, FolderOpen, FileArchive, AlertCircle, Check, UploadCloud, X } from 'lucide-react'

function Hauls() {
  const [hauls, setHauls] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [deleteConfirm, setDeleteConfirm] = useState(null)
  const [successMessage, setSuccessMessage] = useState(null)
  const [uploadModalOpen, setUploadModalOpen] = useState(false)
  const [uploadFile, setUploadFile] = useState(null)
  const [uploading, setUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [loadConfirm, setLoadConfirm] = useState(null)
  const [clearBeforeLoad, setClearBeforeLoad] = useState(false)
  const fileInputRef = useRef(null)
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()

  const fetchHauls = async () => {
    try {
      const res = await fetch('/api/store/hauls')
      if (res.ok) {
        const data = await res.json()
        setHauls(data.hauls || [])
        setError(null)
      } else {
        const text = await res.text()
        setError('Failed to load hauls: ' + text)
      }
    } catch (err) {
      setError('Failed to load hauls: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchHauls()
    // Refresh every 30 seconds to pick up new archives
    const interval = setInterval(fetchHauls, 30000)
    return () => clearInterval(interval)
  }, [])

  const handleCreateHaul = () => {
    navigate('/store/save')
  }

  const handleDownload = (filename) => {
    window.location.href = `/api/downloads/${filename}`
  }

  const handleDelete = async (filename) => {
    try {
      const res = await fetch(`/api/store/hauls/${filename}`, {
        method: 'DELETE'
      })

      if (res.ok) {
        setSuccessMessage(`Deleted ${filename}`)
        setTimeout(() => setSuccessMessage(null), 3000)
        await fetchHauls()
      } else {
        const data = await res.json()
        setError('Failed to delete haul: ' + (data.error || 'Unknown error'))
      }
    } catch (err) {
      setError('Failed to delete haul: ' + err.message)
    } finally {
      setDeleteConfirm(null)
    }
  }

  // Handle file selection for upload
  const handleFileSelect = (e) => {
    const file = e.target.files[0]
    if (file) {
      if (!file.name.toLowerCase().endsWith('.tar.zst')) {
        setError('Only .tar.zst files are allowed')
        return
      }
      setUploadFile(file)
      setError(null)
    }
  }

  // Handle upload submission
  const handleUpload = async () => {
    if (!uploadFile) {
      setError('Please select a file to upload')
      return
    }

    setUploading(true)
    setUploadProgress(0)
    setError(null)

    try {
      const formData = new FormData()
      formData.append('file', uploadFile)

      const xhr = new XMLHttpRequest()

      // Track upload progress
      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          const percentComplete = (e.loaded / e.total) * 100
          setUploadProgress(percentComplete)
        }
      })

      // Handle completion
      xhr.addEventListener('load', () => {
        setUploading(false)
        if (xhr.status === 201 || xhr.status === 200) {
          setSuccessMessage(`Uploaded ${uploadFile.name}`)
          setTimeout(() => setSuccessMessage(null), 3000)
          setUploadFile(null)
          setUploadModalOpen(false)
          setUploadProgress(0)
          if (fileInputRef.current) {
            fileInputRef.current.value = ''
          }
          fetchHauls()
        } else {
          let errorMsg = 'Upload failed'
          try {
            const data = JSON.parse(xhr.responseText)
            errorMsg = data.message || data.error || errorMsg
          } catch {
            errorMsg = `Upload failed: ${xhr.statusText}`
          }
          setError(errorMsg)
        }
      })

      // Handle error
      xhr.addEventListener('error', () => {
        setUploading(false)
        setError('Network error during upload')
      })

      // Send the request
      xhr.open('POST', '/api/store/hauls/upload')
      xhr.send(formData)
    } catch (err) {
      setUploading(false)
      setError('Failed to upload file: ' + err.message)
    }
  }

  // Handle loading a specific haul
  const handleLoadHaul = async (filename, clearStore) => {
    try {
      const requestPayload = {
        filenames: [filename],
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
      setLoadConfirm(null)
      navigate(`/jobs/${data.jobId}`)
    } catch (err) {
      setError(err.message)
    }
  }

  // Format bytes to human readable size
  const formatSize = (bytes) => {
    if (!bytes || bytes === 0) return '-'
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    let size = bytes
    let unitIndex = 0
    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024
      unitIndex++
    }
    return `${size.toFixed(size < 10 ? 1 : 0)} ${units[unitIndex]}`
  }

  // Format date to readable string
  const formatDate = (dateStr) => {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleString()
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Hauls</h1>
          <p className="page-subtitle">Manage haul archive files (.tar.zst)</p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button className="btn" onClick={handleCreateHaul}>
            <FileArchive size={16} style={{ marginRight: '0.25rem' }} />
            Create Haul
          </button>
          <button className="btn" onClick={() => setUploadModalOpen(true)}>
            <UploadCloud size={16} style={{ marginRight: '0.25rem' }} />
            Upload Haul
          </button>
          <button className="btn" onClick={() => { setLoading(true); fetchHauls(); }}>
            <RefreshCw size={16} className={loading ? 'spin' : ''} />
          </button>
        </div>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {successMessage && (
        <div className="card" style={{ borderColor: 'var(--accent-green)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-green)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Check size={18} />
            Success
          </div>
          <p style={{ color: 'var(--text-secondary)' }}>{successMessage}</p>
        </div>
      )}

      <div className="card">
        {loading && hauls.length === 0 ? (
          <div className="loading">Loading hauls...</div>
        ) : hauls.length === 0 ? (
          <div className="empty-state">
            <div className="empty-state-icon">
              <FolderOpen size={48} style={{ color: 'var(--text-muted)' }} />
            </div>
            <div className="empty-state-text">No hauls found</div>
            <p style={{ color: 'var(--text-secondary)', marginTop: '0.5rem' }}>
              Hauls are .tar.zst archive files created from your store contents.
            </p>
            <div style={{ marginTop: '1rem', display: 'flex', gap: '0.5rem', justifyContent: 'center' }}>
              <button className="btn btn-primary" onClick={handleCreateHaul}>
                <FileArchive size={16} style={{ marginRight: '0.25rem' }} />
                Create Your First Haul
              </button>
              <button className="btn" onClick={() => setUploadModalOpen(true)}>
                <UploadCloud size={16} style={{ marginRight: '0.25rem' }} />
                Upload Existing Haul
              </button>
            </div>
          </div>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th style={{ width: '50px' }}></th>
                <th>Filename</th>
                <th style={{ width: '100px' }}>Size</th>
                <th style={{ width: '180px' }}>Created</th>
                <th style={{ width: '220px' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {hauls.map((haul) => (
                <tr key={haul.name}>
                  <td>
                    <Package size={18} style={{ color: 'var(--accent-amber)' }} />
                  </td>
                  <td className="primary">
                    <div style={{ fontWeight: 500 }}>{haul.name}</div>
                  </td>
                  <td style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                    {formatSize(haul.size)}
                  </td>
                  <td style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                    {formatDate(haul.modified)}
                  </td>
                  <td>
                    <div style={{ display: 'flex', gap: '0.25rem' }}>
                      <button
                        className="btn btn-sm"
                        onClick={() => handleDownload(haul.name)}
                        title="Download"
                      >
                        <Download size={14} />
                      </button>
                      <button
                        className="btn btn-sm"
                        onClick={() => setLoadConfirm(haul.name)}
                        title="Load into store"
                        style={{ color: 'var(--accent-green)' }}
                      >
                        <Upload size={14} />
                      </button>
                      {deleteConfirm === haul.name ? (
                        <>
                          <button
                            className="btn btn-sm"
                            onClick={() => setDeleteConfirm(null)}
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            Cancel
                          </button>
                          <button
                            className="btn btn-sm btn-primary"
                            onClick={() => handleDelete(haul.name)}
                            style={{ backgroundColor: 'var(--accent-red)', borderColor: 'var(--accent-red)' }}
                          >
                            Confirm
                          </button>
                        </>
                      ) : (
                        <button
                          className="btn btn-sm"
                          onClick={() => setDeleteConfirm(haul.name)}
                          title="Delete"
                          style={{ color: 'var(--accent-red)' }}
                        >
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

      {/* Info Panel */}
      <div className="card" style={{ marginTop: '1.5rem' }}>
        <div className="card-title">About Hauls</div>
        <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
          <p style={{ marginBottom: '0.5rem' }}>
            <strong>Hauls</strong> are compressed archive files (.tar.zst) that contain the entire contents
            of your hauler store. They are used for transferring content to air-gapped environments.
          </p>
          <p style={{ marginBottom: '0.5rem' }}>
            <strong>Workflow:</strong>
          </p>
          <ol style={{ paddingLeft: '1.5rem', marginBottom: '0.5rem' }}>
            <li>Add content to your store using the <NavLink to="/store" style={{ color: 'var(--accent-amber)' }}>Store</NavLink> page</li>
            <li><strong>Create a haul</strong> - Save your store as a .tar.zst archive</li>
            <li><strong>Transfer</strong> - Move the archive to your air-gapped system</li>
            <li><strong>Load the haul</strong> - Import the archive into the destination store</li>
          </ol>
          <p>
            View what's currently in your store on the <NavLink to="/store/contents" style={{ color: 'var(--accent-amber)' }}>Store Contents</NavLink> page.
          </p>
        </div>
      </div>

      {/* Warning Panel */}
      <div className="card" style={{ marginTop: '1.5rem', borderColor: 'var(--accent-amber-dim)' }}>
        <div className="card-title" style={{ color: 'var(--accent-amber)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <AlertCircle size={18} />
          Important Notes
        </div>
        <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
          <p style={{ marginBottom: '0.5rem' }}>
            <strong>Storage Space:</strong> Ensure you have sufficient disk space before creating a haul.
            Archives can be very large depending on your store contents.
          </p>
          <p style={{ marginBottom: '0' }}>
            <strong>Loading Archives:</strong> Loading a haul will <strong>merge</strong> its contents with
            your existing store. It will not replace or clear existing content.
          </p>
        </div>
      </div>

      {/* Upload Modal */}
      {uploadModalOpen && (
        <div className="modal-overlay" onClick={() => !uploading && setUploadModalOpen(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '500px' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
              <h2 style={{ margin: 0, fontSize: '1.25rem' }}>Upload Haul</h2>
              <button
                className="btn btn-sm"
                onClick={() => !uploading && setUploadModalOpen(false)}
                disabled={uploading}
                style={{ padding: '0.25rem 0.5rem' }}
              >
                <X size={16} />
              </button>
            </div>

            <div style={{ marginBottom: '1rem' }}>
              <label className="form-label">Select .tar.zst file</label>
              <input
                ref={fileInputRef}
                type="file"
                accept=".tar.zst"
                onChange={handleFileSelect}
                disabled={uploading}
                className="form-input"
              />
            </div>

            {uploadFile && (
              <div style={{ marginBottom: '1rem', padding: '0.75rem', backgroundColor: 'var(--bg-primary)', borderRadius: '4px' }}>
                <div style={{ fontSize: '0.9rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <Package size={16} style={{ color: 'var(--accent-amber)' }} />
                  <span style={{ fontWeight: 500 }}>{uploadFile.name}</span>
                </div>
                <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>
                  {formatSize(uploadFile.size)}
                </div>
              </div>
            )}

            {uploading && (
              <div style={{ marginBottom: '1rem' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.25rem' }}>
                  <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Uploading...</span>
                  <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>{uploadProgress.toFixed(0)}%</span>
                </div>
                <div style={{ height: '4px', backgroundColor: 'var(--border-color)', borderRadius: '2px', overflow: 'hidden' }}>
                  <div
                    style={{
                      height: '100%',
                      width: `${uploadProgress}%`,
                      backgroundColor: 'var(--accent-green)',
                      transition: 'width 0.2s ease'
                    }}
                  />
                </div>
              </div>
            )}

            <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end' }}>
              <button
                className="btn"
                onClick={() => !uploading && setUploadModalOpen(false)}
                disabled={uploading}
              >
                Cancel
              </button>
              <button
                className="btn btn-primary"
                onClick={handleUpload}
                disabled={uploading || !uploadFile}
              >
                {uploading ? 'Uploading...' : 'Upload'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Load Confirmation Modal */}
      {loadConfirm && (
        <div className="modal-overlay" onClick={() => { setLoadConfirm(null); setClearBeforeLoad(false); }}>
          <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '450px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '1rem' }}>
              <Upload size={24} style={{ color: 'var(--accent-green)' }} />
              <h2 style={{ margin: 0, fontSize: '1.25rem' }}>Load Haul?</h2>
            </div>
            <p style={{ color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
              Load <strong>{loadConfirm}</strong> into the store?
            </p>
            <p style={{ color: 'var(--text-secondary)', marginBottom: '1.5rem', fontSize: '0.9rem' }}>
              This will merge the archive contents with your existing store.
            </p>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', marginBottom: '1.5rem' }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer', fontSize: '0.9rem' }}>
                <input
                  type="checkbox"
                  checked={clearBeforeLoad}
                  onChange={(e) => setClearBeforeLoad(e.target.checked)}
                  style={{ cursor: 'pointer' }}
                />
                <span>Clear store before loading</span>
              </label>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginLeft: '1.5rem' }}>
                Removes all existing content from the store before loading this archive.
              </div>
            </div>
            <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end' }}>
              <button
                className="btn"
                onClick={() => { setLoadConfirm(null); setClearBeforeLoad(false); }}
              >
                Cancel
              </button>
              <button
                className="btn btn-primary"
                onClick={() => handleLoadHaul(loadConfirm, clearBeforeLoad)}
                style={clearBeforeLoad ? { backgroundColor: 'var(--accent-amber)', borderColor: 'var(--accent-amber)', color: 'var(--bg-primary)' } : {}}
              >
                {clearBeforeLoad ? 'Clear & Load' : 'Load (Merge)'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default Hauls
