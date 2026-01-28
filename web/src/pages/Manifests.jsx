import { useState, useEffect, useRef } from 'react'
import { NavLink } from 'react-router-dom'
import * as yaml from 'js-yaml'
import { X, AlertTriangle, Search, Clipboard, RefreshCw, Edit, Trash2, Download } from 'lucide-react'
import { useModal } from '../components/Modal.jsx'

// Manifest templates based on hauler content.hauler.cattle.io/v1
const MANIFEST_TEMPLATES = {
  image: `apiVersion: content.hauler.cattle.io/v1
kind: Image
name: busybox
# Add optional platform specification
# platforms:
#   - linux/amd64
#   - linux/arm64
`,
  chart: `apiVersion: content.hauler.cattle.io/v1
kind: Chart
name: nginx-ingress
# Optional: specify chart version
# version: 4.8.3
# Optional: specify repo URL
# repo: https://kubernetes.github.io/ingress-nginx
`,
  file: `apiVersion: content.hauler.cattle.io/v1
kind: File
name: my-file
# Optional: specify source URL
# path: https://example.com/file.tar.gz
`
}

const MULTIPLE_RESOURCES_TEMPLATE = `apiVersion: content.hauler.cattle.io/v1
kind: Images
items:
  - name: busybox
  - name: nginx
    # platforms:
    #   - linux/amd64
---
apiVersion: content.hauler.cattle.io/v1
kind: Charts
items:
  - name: nginx-ingress
---
apiVersion: content.hauler.cattle.io/v1
kind: Files
items:
  - name: my-config
`

function Manifests() {
  const { confirm } = useModal()
  const [manifests, setManifests] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [showEditor, setShowEditor] = useState(false)
  const [editingManifest, setEditingManifest] = useState(null)
  const [searchQuery, setSearchQuery] = useState('')

  // Editor form state
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [yamlContent, setYamlContent] = useState('')
  const [tags, setTags] = useState('')
  const [yamlError, setYamlError] = useState(null)

  const textareaRef = useRef(null)

  const fetchManifests = async () => {
    try {
      const res = await fetch('/api/manifests')
      if (res.ok) {
        const data = await res.json()
        setManifests(data)
      } else {
        setError('Failed to load manifests')
      }
    } catch (err) {
      setError('Failed to load manifests: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchManifests()
  }, [])

  const handleCreateNew = () => {
    setEditingManifest(null)
    setName('')
    setDescription('')
    setYamlContent(MANIFEST_TEMPLATES.image)
    setTags('')
    setYamlError(null)
    setShowEditor(true)
  }

  const handleEdit = (manifest) => {
    setEditingManifest(manifest)
    setName(manifest.name)
    setDescription(manifest.description || '')
    setYamlContent(manifest.yamlContent)
    setTags(manifest.tags ? manifest.tags.join(', ') : '')
    setYamlError(null)
    setShowEditor(true)
  }

  const handleDelete = async (id, name) => {
    const confirmed = await confirm('Delete Manifest', `Are you sure you want to delete "${name}"?`)
    if (!confirmed) {
      return
    }

    try {
      const res = await fetch(`/api/manifests/${id}`, {
        method: 'DELETE'
      })

      if (res.ok) {
        await fetchManifests()
      } else {
        const data = await res.json()
        setError(data.message || 'Failed to delete manifest')
      }
    } catch (err) {
      setError('Failed to delete manifest: ' + err.message)
    }
  }

  const handleDownload = async (id, filename) => {
    try {
      const res = await fetch(`/api/manifests/${id}/download`)
      if (res.ok) {
        const blob = await res.blob()
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = filename.replace(/\s+/g, '_') + '.yaml'
        document.body.appendChild(a)
        a.click()
        window.URL.revokeObjectURL(url)
        document.body.removeChild(a)
      } else {
        setError('Failed to download manifest')
      }
    } catch (err) {
      setError('Failed to download manifest: ' + err.message)
    }
  }

  const handleSave = async () => {
    setError(null)
    setYamlError(null)

    // Validate name
    if (!name.trim()) {
      setError('Name is required')
      return
    }

    // Validate YAML content
    if (!yamlContent.trim()) {
      setError('YAML content is required')
      return
    }

    // Client-side YAML validation
    try {
      yaml.load(yamlContent)
    } catch (err) {
      setYamlError('Invalid YAML: ' + err.message)
      return
    }

    // Parse tags
    const tagsArray = tags
      .split(',')
      .map(t => t.trim())
      .filter(t => t.length > 0)

    const payload = {
      name: name.trim(),
      description: description.trim(),
      yamlContent: yamlContent.trim(),
      tags: tagsArray
    }

    try {
      const url = editingManifest
        ? `/api/manifests/${editingManifest.id}`
        : '/api/manifests'
      const method = editingManifest ? 'PUT' : 'POST'

      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })

      const data = await res.json()

      if (res.ok) {
        await fetchManifests()
        setShowEditor(false)
        setEditingManifest(null)
      } else {
        if (res.status === 409) {
          setError('A manifest with this name already exists')
        } else {
          setError(data.message || 'Failed to save manifest')
        }
      }
    } catch (err) {
      setError('Failed to save manifest: ' + err.message)
    }
  }

  const handleLoadTemplate = (type) => {
    setYamlContent(MANIFEST_TEMPLATES[type])
    setYamlError(null)
    if (textareaRef.current) {
      textareaRef.current.focus()
    }
  }

  const handleLoadMultipleTemplate = () => {
    setYamlContent(MULTIPLE_RESOURCES_TEMPLATE)
    setYamlError(null)
    if (textareaRef.current) {
      textareaRef.current.focus()
    }
  }

  const handleYamlChange = (e) => {
    const content = e.target.value
    setYamlContent(content)
    setYamlError(null)

    // Real-time YAML validation (debounced could be better, but keeping it simple)
    if (content.trim()) {
      try {
        yaml.load(content)
      } catch {
        // Don't show error while typing, only mark as invalid
        // We'll show the error on save
      }
    }
  }

  // Filter manifests based on search
  const filteredManifests = manifests.filter(m => {
    const query = searchQuery.toLowerCase()
    return (
      m.name.toLowerCase().includes(query) ||
      (m.description && m.description.toLowerCase().includes(query)) ||
      (m.tags && m.tags.some(t => t.toLowerCase().includes(query)))
    )
  })

  const formatDate = (dateStr) => {
    return new Date(dateStr).toLocaleString()
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Manifests</h1>
          <p className="page-subtitle">Create and manage hauler manifests</p>
        </div>
        <button className="btn btn-primary" onClick={handleCreateNew}>
          + New Manifest
        </button>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
          <button className="btn btn-sm" onClick={() => setError(null)}>Dismiss</button>
        </div>
      )}

      {/* Editor Modal */}
      {showEditor && (
        <div className="modal-overlay" onClick={async () => {
          const confirmed = await confirm('Close Editor', 'Close editor without saving?')
          if (confirmed) {
            setShowEditor(false)
            setEditingManifest(null)
          }
        }}>
          <div className="modal-content large" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2 className="modal-title">
                {editingManifest ? 'Edit Manifest' : 'New Manifest'}
              </h2>
              <button
                className="btn btn-sm"
                onClick={async () => {
                  const confirmed = await confirm('Close Editor', 'Close editor without saving?')
                  if (confirmed) {
                    setShowEditor(false)
                    setEditingManifest(null)
                  }
                }}
              >
                <X size={18} />
              </button>
            </div>

            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">Name *</label>
                <input
                  className="form-input"
                  placeholder="my-manifest"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>

              <div className="form-group">
                <label className="form-label">Description</label>
                <input
                  className="form-input"
                  placeholder="Optional description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
              </div>

              <div className="form-group">
                <label className="form-label">Tags</label>
                <input
                  className="form-input"
                  placeholder="image, production, v1.0.0 (comma-separated)"
                  value={tags}
                  onChange={(e) => setTags(e.target.value)}
                />
              </div>

              <div className="form-group">
                <label className="form-label">YAML Content *</label>
                {!editingManifest && (
                  <div style={{ marginBottom: '0.5rem', display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={() => handleLoadTemplate('image')}
                    >
                      Load Image Template
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={() => handleLoadTemplate('chart')}
                    >
                      Load Chart Template
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={() => handleLoadTemplate('file')}
                    >
                      Load File Template
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={handleLoadMultipleTemplate}
                    >
                      Load Multi-Resource Template
                    </button>
                  </div>
                )}
                <div className="editor-container">
                  <textarea
                    ref={textareaRef}
                    className="yaml-editor"
                    value={yamlContent}
                    onChange={handleYamlChange}
                    style={{
                      width: '100%',
                      minHeight: '400px',
                      fontFamily: 'var(--font-mono)',
                      fontSize: '0.85rem',
                      lineHeight: '1.6',
                      padding: '1rem',
                      backgroundColor: '#0a0a0a',
                      border: '1px solid var(--border-color)',
                      borderRadius: '2px',
                      color: 'var(--text-primary)',
                      resize: 'vertical',
                      tabSize: 2
                    }}
                    placeholder="Enter your YAML manifest here..."
                    spellCheck="false"
                  />
                </div>
                {yamlError && (
                  <div style={{ color: 'var(--accent-red)', fontSize: '0.85rem', marginTop: '0.5rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    <AlertTriangle size={14} />
                    {yamlError}
                  </div>
                )}
              </div>
            </div>

            <div className="modal-footer">
              <button
                className="btn"
                onClick={async () => {
                  const confirmed = await confirm('Close Editor', 'Close editor without saving?')
                  if (confirmed) {
                    setShowEditor(false)
                    setEditingManifest(null)
                  }
                }}
              >
                Cancel
              </button>
              <button className="btn btn-primary" onClick={handleSave}>
                {editingManifest ? 'Update' : 'Create'} Manifest
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Manifests List */}
      {loading ? (
        <div className="loading">Loading manifests...</div>
      ) : filteredManifests.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state-icon">
            {searchQuery ? <Search size={48} style={{ color: 'var(--text-muted)' }} /> : <Clipboard size={48} style={{ color: 'var(--text-muted)' }} />}
          </div>
          <div className="empty-state-text">
            {searchQuery ? 'No matching manifests found' : 'No manifests yet'}
          </div>
          {!searchQuery && (
            <button className="btn btn-primary" onClick={handleCreateNew}>
              Create your first manifest
            </button>
          )}
        </div>
      ) : (
        <div className="card">
          <div className="card-title">
            Saved Manifests ({filteredManifests.length})
          </div>

          <div className="form-group" style={{ marginBottom: '1rem' }}>
            <input
              className="form-input"
              placeholder="Search by name, description, or tags..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>

          <div style={{ display: 'grid', gap: '1rem' }}>
            {filteredManifests.map((manifest) => (
              <div
                key={manifest.id}
                className="manifest-card"
                style={{
                  border: '1px solid var(--border-color)',
                  borderRadius: '4px',
                  padding: '1rem',
                  backgroundColor: 'var(--bg-secondary)'
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.25rem' }}>
                      <h3 style={{ margin: 0, fontSize: '1rem', fontWeight: 500, color: 'var(--text-primary)' }}>
                        {manifest.name}
                      </h3>
                      <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                        #{manifest.id}
                      </span>
                    </div>

                    {manifest.description && (
                      <p style={{
                        margin: '0.25rem 0 0.5rem',
                        fontSize: '0.85rem',
                        color: 'var(--text-secondary)'
                      }}>
                        {manifest.description}
                      </p>
                    )}

                    {manifest.tags && manifest.tags.length > 0 && (
                      <div style={{ display: 'flex', gap: '0.25rem', flexWrap: 'wrap', marginTop: '0.5rem' }}>
                        {manifest.tags.map((tag, i) => (
                          <span
                            key={i}
                            style={{
                              fontSize: '0.75rem',
                              padding: '0.1rem 0.4rem',
                              backgroundColor: 'var(--bg-tertiary)',
                              borderRadius: '2px',
                              color: 'var(--text-secondary)'
                            }}
                          >
                            {tag}
                          </span>
                        ))}
                      </div>
                    )}

                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.5rem' }}>
                      Updated: {formatDate(manifest.updatedAt)}
                    </div>
                  </div>

                  <div style={{ display: 'flex', gap: '0.5rem', marginLeft: '1rem' }}>
                    <button
                      className="btn btn-sm"
                      onClick={() => handleDownload(manifest.id, manifest.name)}
                      title="Download as YAML file"
                    >
                      <Download size={16} />
                    </button>
                    <NavLink
                      to={`/store/sync/${manifest.id}`}
                      className="btn btn-sm"
                      title="Use this manifest for sync"
                      style={{ color: 'var(--accent-green)' }}
                    >
                      <RefreshCw size={16} />
                    </NavLink>
                    <button
                      className="btn btn-sm"
                      onClick={() => handleEdit(manifest)}
                      title="Edit manifest"
                    >
                      <Edit size={16} />
                    </button>
                    <button
                      className="btn btn-sm"
                      onClick={() => handleDelete(manifest.id, manifest.name)}
                      title="Delete manifest"
                      style={{ color: 'var(--accent-red)' }}
                    >
                      <Trash2 size={16} />
                    </button>
                  </div>
                </div>

                {/* YAML Preview */}
                <details style={{ marginTop: '0.75rem' }}>
                  <summary
                    style={{
                      cursor: 'pointer',
                      fontSize: '0.8rem',
                      color: 'var(--accent-amber)',
                      userSelect: 'none'
                    }}
                  >
                    Show YAML Preview
                  </summary>
                  <pre
                    style={{
                      marginTop: '0.5rem',
                      padding: '0.5rem',
                      backgroundColor: 'var(--bg-primary)',
                      border: '1px solid var(--border-color)',
                      borderRadius: '2px',
                      fontSize: '0.75rem',
                      overflow: 'auto',
                      maxHeight: '200px'
                    }}
                  >
                    {manifest.yamlContent}
                  </pre>
                </details>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Help Panel */}
      {!showEditor && (
        <div className="card" style={{ marginTop: '1.5rem' }}>
          <div className="card-title">About Hauler Manifests</div>
          <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
            <p style={{ margin: 0 }}>
              Hauler manifests use <code>content.hauler.cattle.io/v1</code> API version to define
              images, charts, and files for airgap operations. Use these manifests with the
              <NavLink to="/store/sync" style={{ color: 'var(--accent-amber)' }}> Store Sync</NavLink>
              operation to populate your store.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}

export default Manifests
