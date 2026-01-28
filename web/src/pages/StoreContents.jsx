import { useState, useEffect } from 'react'
import { NavLink } from 'react-router-dom'
import { Image, BarChart3, FileText, RefreshCw, Search, Package, Folder, FileText as FileIcon } from 'lucide-react'

function StoreContents() {
  const [storeInfo, setStoreInfo] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [filter, setFilter] = useState('all') // all, images, charts, files
  const [searchQuery, setSearchQuery] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [rescanning, setRescanning] = useState(false)
  const [rescanMessage, setRescanMessage] = useState(null)

  const fetchStoreInfo = async () => {
    try {
      const res = await fetch('/api/store/info')
      if (res.ok) {
        const data = await res.json()
        setStoreInfo(data)
        setError(null)
      } else {
        const text = await res.text()
        setError('Failed to load store info: ' + text)
      }
    } catch (err) {
      setError('Failed to load store info: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleRescan = async () => {
    setRescanning(true)
    setRescanMessage(null)
    try {
      const res = await fetch('/api/store/rescan', { method: 'POST' })
      if (res.ok) {
        const data = await res.json()
        setRescanMessage({ type: 'success', text: data.message || `Rescan complete: tracked ${data.itemsFound} items` })
        // Refresh store info after rescan
        setTimeout(fetchStoreInfo, 500)
      } else {
        const data = await res.json()
        setRescanMessage({ type: 'error', text: data.message || 'Rescan failed' })
      }
    } catch (err) {
      setRescanMessage({ type: 'error', text: 'Rescan failed: ' + err.message })
    } finally {
      setRescanning(false)
    }
  }

  useEffect(() => {
    fetchStoreInfo()
  }, [])

  // Auto-refresh every 30 seconds if enabled
  useEffect(() => {
    if (!autoRefresh) return
    const interval = setInterval(fetchStoreInfo, 30000)
    return () => clearInterval(interval)
  }, [autoRefresh])

  const handleRefresh = () => {
    setLoading(true)
    fetchStoreInfo()
  }

  // Format bytes to human readable size
  const formatSize = (bytes) => {
    if (!bytes || bytes === 0) return '-'
    const units = ['B', 'KB', 'MB', 'GB']
    let size = bytes
    let unitIndex = 0
    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024
      unitIndex++
    }
    return `${size.toFixed(size < 10 ? 1 : 0)} ${units[unitIndex]}`
  }

  // Truncate digest for display
  const truncateDigest = (digest) => {
    if (!digest) return '-'
    if (digest.length <= 16) return digest
    return digest.substring(0, 16) + '...'
  }

  // Filter and search artifacts
  const filteredArtifacts = []
  if (storeInfo) {
    if (filter === 'all' || filter === 'images') {
      storeInfo.images?.forEach(img => {
        if (!searchQuery || img.name?.toLowerCase().includes(searchQuery.toLowerCase())) {
          filteredArtifacts.push({ type: 'image', ...img })
        }
      })
    }
    if (filter === 'all' || filter === 'charts') {
      storeInfo.charts?.forEach(chart => {
        const searchStr = `${chart.name} ${chart.version}`.toLowerCase()
        if (!searchQuery || searchStr.includes(searchQuery.toLowerCase())) {
          filteredArtifacts.push({ type: 'chart', ...chart })
        }
      })
    }
    if (filter === 'all' || filter === 'files') {
      storeInfo.files?.forEach(file => {
        if (!searchQuery || file.name?.toLowerCase().includes(searchQuery.toLowerCase())) {
          filteredArtifacts.push({ type: 'file', ...file })
        }
      })
    }
  }

  // Count total artifacts
  const totalCount = {
    images: storeInfo?.images?.length || 0,
    charts: storeInfo?.charts?.length || 0,
    files: storeInfo?.files?.length || 0
  }

  const getTypeIcon = (type) => {
    switch (type) {
      case 'image': return <Image size={18} style={{ color: 'var(--accent-blue)' }} />
      case 'chart': return <BarChart3 size={18} style={{ color: 'var(--accent-green)' }} />
      case 'file': return <FileText size={18} style={{ color: 'var(--accent-amber)' }} />
      default: return <Package size={18} />
    }
  }

  const getTypeLabel = (type) => {
    return type.charAt(0).toUpperCase() + type.slice(1)
  }

  const getSourceBadge = (sourceHaul) => {
    if (!sourceHaul) {
      return (
        <span style={{
          fontSize: '0.7rem',
          padding: '2px 6px',
          borderRadius: '4px',
          background: 'var(--border-color)',
          color: 'var(--text-muted)'
        }}>
          Before tracking
        </span>
      )
    }
    return (
      <span style={{
        fontSize: '0.7rem',
        padding: '2px 6px',
        borderRadius: '4px',
        background: 'var(--accent-blue)',
        color: 'white',
        maxWidth: '120px',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        display: 'inline-block'
      }} title={sourceHaul}>
        {sourceHaul}
      </span>
    )
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Store Contents</h1>
          <p className="page-subtitle">Browse contents of your content store</p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
          <button
            className="btn btn-sm"
            onClick={handleRescan}
            disabled={rescanning || loading}
            title="Rescan store to update provenance tracking"
          >
            <FileIcon size={14} style={{ marginRight: '0.25rem' }} />
            {rescanning ? 'Rescanning...' : 'Rescan'}
          </button>
          <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
              style={{ cursor: 'pointer' }}
            />
            Auto-refresh
          </label>
          <button className="btn" onClick={handleRefresh} disabled={loading}>
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

      {rescanMessage && (
        <div className="card" style={{
          borderColor: rescanMessage.type === 'success' ? 'var(--accent-green)' : 'var(--accent-red)',
          marginBottom: '1rem'
        }}>
          <div className="card-title" style={{ color: rescanMessage.type === 'success' ? 'var(--accent-green)' : 'var(--accent-red)' }}>
            {rescanMessage.type === 'success' ? 'Success' : 'Error'}
          </div>
          <p style={{ color: 'var(--text-secondary)' }}>{rescanMessage.text}</p>
        </div>
      )}

      <div className="card">
        {/* Filter Tabs */}
        <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
          <button
            className={`btn btn-sm ${filter === 'all' ? 'btn-primary' : ''}`}
            onClick={() => setFilter('all')}
          >
            All ({totalCount.images + totalCount.charts + totalCount.files})
          </button>
          <button
            className={`btn btn-sm ${filter === 'images' ? 'btn-primary' : ''}`}
            onClick={() => setFilter('images')}
          >
            <Image size={14} style={{ marginRight: '0.25rem' }} />
            Images ({totalCount.images})
          </button>
          <button
            className={`btn btn-sm ${filter === 'charts' ? 'btn-primary' : ''}`}
            onClick={() => setFilter('charts')}
          >
            <BarChart3 size={14} style={{ marginRight: '0.25rem' }} />
            Charts ({totalCount.charts})
          </button>
          <button
            className={`btn btn-sm ${filter === 'files' ? 'btn-primary' : ''}`}
            onClick={() => setFilter('files')}
          >
            <FileText size={14} style={{ marginRight: '0.25rem' }} />
            Files ({totalCount.files})
          </button>
        </div>

        {/* Search */}
        <div className="form-group" style={{ marginBottom: '1rem' }}>
          <div style={{ position: 'relative' }}>
            <Search size={16} style={{ position: 'absolute', left: '0.75rem', top: '0.75rem', color: 'var(--text-muted)' }} />
            <input
              className="form-input"
              placeholder="Search by name or reference..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              style={{ paddingLeft: '2.5rem' }}
            />
          </div>
        </div>

        {/* Results */}
        {loading && !storeInfo ? (
          <div className="loading">Loading store contents...</div>
        ) : filteredArtifacts.length === 0 ? (
          <div className="empty-state">
            <div className="empty-state-icon">
              <Folder size={48} style={{ color: 'var(--text-muted)' }} />
            </div>
            <div className="empty-state-text">
              {searchQuery ? 'No matching artifacts found' : 'No artifacts in store'}
            </div>
            {!searchQuery && (
              <p style={{ color: 'var(--text-secondary)', marginTop: '0.5rem' }}>
                Use the <NavLink to="/store" style={{ color: 'var(--accent-amber)' }}>Store</NavLink> page to add content
              </p>
            )}
          </div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table className="data-table">
              <thead>
                <tr>
                  <th style={{ width: '80px' }}>Type</th>
                  <th>Name / Reference</th>
                  <th>Digest</th>
                  <th style={{ width: '140px' }}>Source Haul</th>
                  <th style={{ width: '100px' }}>Size</th>
                </tr>
              </thead>
              <tbody>
                {filteredArtifacts.map((artifact, index) => (
                  <tr key={`${artifact.type}-${index}`}>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                        {getTypeIcon(artifact.type)}
                        <span style={{ fontSize: '0.8rem' }}>{getTypeLabel(artifact.type)}</span>
                      </div>
                    </td>
                    <td className="primary">
                      <div>
                        <div style={{ fontWeight: 500 }}>{artifact.name || '-'}</div>
                        {artifact.version && (
                          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                            v{artifact.version}
                          </div>
                        )}
                      </div>
                    </td>
                    <td>
                      <code style={{ fontSize: '0.75rem' }}>
                        {truncateDigest(artifact.digest)}
                      </code>
                    </td>
                    <td>
                      {getSourceBadge(artifact.sourceHaul)}
                    </td>
                    <td style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                      {formatSize(artifact.size)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Info Panel */}
      <div className="card" style={{ marginTop: '1.5rem' }}>
        <div className="card-title">About Store Contents</div>
        <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
          <p style={{ marginBottom: '0.5rem' }}>
            This page displays all artifacts currently stored in your hauler content store.
            The store contains container images, Helm charts, and files that can be served
            or extracted for airgap deployments.
          </p>
          <p style={{ marginBottom: '0.5rem' }}>
            <strong>Source Haul:</strong> Shows which archive file each item was loaded from.
            Items loaded before tracking was enabled show "Before tracking". Use the <strong>Rescan</strong> button
            to rebuild this information from the current store contents.
          </p>
          <p>
            <strong>Tip:</strong> Use the <NavLink to="/store" style={{ color: 'var(--accent-amber)' }}>Store</NavLink> page
            {' '}to add more content, the <NavLink to="/store/sync" style={{ color: 'var(--accent-amber)' }}>Sync</NavLink> page
            {' '}to populate from manifests, or the <NavLink to="/hauls" style={{ color: 'var(--accent-amber)' }}>Hauls</NavLink> page
            {' '}to manage archive files.
          </p>
        </div>
      </div>
    </div>
  )
}

export default StoreContents
