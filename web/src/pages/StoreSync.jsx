import { useState, useEffect } from 'react'
import { useNavigate, NavLink, useParams } from 'react-router-dom'
import { useJobs } from '../App.jsx'
import { AlertTriangle, X, RefreshCw } from 'lucide-react'

function StoreSync() {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()
  const { manifestId } = useParams()

  const [manifests, setManifests] = useState([])
  const [error, setError] = useState(null)

  // Manifest source selection: 'saved', 'paste', 'filepath'
  const [sourceType, setSourceType] = useState('saved')

  // For saved manifest selection
  const [selectedManifestId, setSelectedManifestId] = useState('')

  // For pasted YAML
  const [pastedYaml, setPastedYaml] = useState('')

  // Multiple filenames list (for -f flag)
  const [fileList, setFileList] = useState(['hauler-manifest.yaml'])

  // Sync options from hauler store sync --help
  const [platform, setPlatform] = useState('')
  const [key, setKey] = useState('')
  const [certificateIdentity, setCertificateIdentity] = useState('')
  const [certificateIdentityRegexp, setCertificateIdentityRegexp] = useState('')
  const [certificateOidcIssuer, setCertificateOidcIssuer] = useState('')
  const [certificateOidcIssuerRegexp, setCertificateOidcIssuerRegexp] = useState('')
  const [certificateGithubWorkflow, setCertificateGithubWorkflow] = useState('')
  const [registry, setRegistry] = useState('')
  const [products, setProducts] = useState('')
  const [productRegistry, setProductRegistry] = useState('')
  const [rewrite, setRewrite] = useState('')
  const [useTlogVerify, setUseTlogVerify] = useState(false)

  // Show advanced options
  const [showAdvanced, setShowAdvanced] = useState(false)

  // Product registry warning
  const [showProductWarning, setShowProductWarning] = useState(false)

  const [submitting, setSubmitting] = useState(false)

  // Fetch manifests on mount and pre-select if manifestId is provided
  useEffect(() => {
    fetchManifests()
    if (manifestId) {
      setSourceType('saved')
      setSelectedManifestId(manifestId)
    }
  }, [manifestId])

  // Show warning when products/product-registry are used
  useEffect(() => {
    setShowProductWarning(products !== '' || productRegistry !== '')
  }, [products, productRegistry])

  const fetchManifests = async () => {
    try {
      const res = await fetch('/api/manifests')
      if (res.ok) {
        const data = await res.json()
        setManifests(data)
      }
    } catch (err) {
      console.error('Failed to load manifests:', err)
    }
  }

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
    setSubmitting(true)

    try {
      // Build request based on source type
      let requestPayload = {
        platform: platform || undefined,
        key: key || undefined,
        certificateIdentity: certificateIdentity || undefined,
        certificateIdentityRegexp: certificateIdentityRegexp || undefined,
        certificateOidcIssuer: certificateOidcIssuer || undefined,
        certificateOidcIssuerRegexp: certificateOidcIssuerRegexp || undefined,
        certificateGithubWorkflow: certificateGithubWorkflow || undefined,
        registry: registry || undefined,
        products: products || undefined,
        productRegistry: productRegistry || undefined,
        rewrite: rewrite || undefined,
        useTlogVerify
      }

      // Filter out undefined values
      Object.keys(requestPayload).forEach(key => {
        if (requestPayload[key] === undefined) {
          delete requestPayload[key]
        }
      })

      if (sourceType === 'saved') {
        if (!selectedManifestId) {
          setError('Please select a saved manifest')
          setSubmitting(false)
          return
        }
        // Get manifest content from API
        const manifestRes = await fetch(`/api/manifests/${selectedManifestId}`)
        if (!manifestRes.ok) {
          setError('Failed to load saved manifest')
          setSubmitting(false)
          return
        }
        const manifestData = await manifestRes.json()
        requestPayload.manifestYaml = manifestData.yamlContent
      } else if (sourceType === 'paste') {
        if (!pastedYaml.trim()) {
          setError('Please paste manifest YAML content')
          setSubmitting(false)
          return
        }
        requestPayload.manifestYaml = pastedYaml.trim()
      } else if (sourceType === 'filepath') {
        // Use non-empty file paths from list
        const validFiles = fileList.filter(f => f.trim() !== '')
        if (validFiles.length === 0) {
          setError('Please provide at least one file path')
          setSubmitting(false)
          return
        }
        requestPayload.filenames = validFiles
      }

      const res = await fetch('/api/store/sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestPayload)
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Sync request failed')
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
          <h1 className="page-title">Store Sync</h1>
          <p className="page-subtitle">Sync content from manifests to the content store</p>
        </div>
        <NavLink to="/store" className="btn">Back to Store</NavLink>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--accent-red)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-red)' }}>Error</div>
          <p style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {showProductWarning && (
        <div className="card" style={{ borderColor: 'var(--accent-amber)', marginBottom: '1rem' }}>
          <div className="card-title" style={{ color: 'var(--accent-amber)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <AlertTriangle size={18} />
            Product Registry Notice
          </div>
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', lineHeight: '1.6' }}>
            <p style={{ marginBottom: '0.5rem' }}>
              <strong>Default Registry Change:</strong> The <code>--products</code> and <code>--product-registry</code>
              flags now default to the RGS Carbide Registry (<code>rgcrprod.azurecr.us</code>) instead of
              the previous registry.
            </p>
            <p style={{ marginBottom: 0 }}>
              If you were using product-based pulls with the old default, you may need to explicitly
              specify the <code>--product-registry</code> to maintain previous behavior.
            </p>
          </div>
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 320px', gap: '1.5rem' }}>
        {/* Main Form */}
        <div>
          <form onSubmit={handleSubmit}>
            {/* Manifest Source Selection */}
            <div className="card" style={{ marginBottom: '1rem' }}>
              <div className="card-title">Manifest Source</div>

              {/* Source Type Tabs */}
              <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.5rem' }}>
                <button
                  type="button"
                  className={`btn btn-sm ${sourceType === 'saved' ? 'btn-primary' : ''}`}
                  onClick={() => setSourceType('saved')}
                  style={{ opacity: sourceType === 'saved' ? 1 : 0.7 }}
                >
                  Saved Manifest
                </button>
                <button
                  type="button"
                  className={`btn btn-sm ${sourceType === 'paste' ? 'btn-primary' : ''}`}
                  onClick={() => setSourceType('paste')}
                  style={{ opacity: sourceType === 'paste' ? 1 : 0.7 }}
                >
                  Paste YAML
                </button>
                <button
                  type="button"
                  className={`btn btn-sm ${sourceType === 'filepath' ? 'btn-primary' : ''}`}
                  onClick={() => setSourceType('filepath')}
                  style={{ opacity: sourceType === 'filepath' ? 1 : 0.7 }}
                >
                  File Paths
                </button>
              </div>

              {/* Saved Manifest Selection */}
              {sourceType === 'saved' && (
                <div className="form-group">
                  <label className="form-label">Select Saved Manifest</label>
                  {manifests.length === 0 ? (
                    <div style={{ padding: '1rem', backgroundColor: 'var(--bg-tertiary)', borderRadius: '4px', textAlign: 'center' }}>
                      <p style={{ margin: 0, color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
                        No saved manifests. <NavLink to="/manifests" style={{ color: 'var(--accent-amber)' }}>Create one</NavLink>
                      </p>
                    </div>
                  ) : (
                    <select
                      className="form-input"
                      value={selectedManifestId}
                      onChange={(e) => setSelectedManifestId(e.target.value)}
                      disabled={submitting}
                      style={{ marginBottom: '0.5rem' }}
                    >
                      <option value="">-- Select a manifest --</option>
                      {manifests.map(m => (
                        <option key={m.id} value={m.id}>{m.name}</option>
                      ))}
                    </select>
                  )}
                  {selectedManifestId && (
                    <NavLink to="/manifests" style={{ fontSize: '0.8rem', color: 'var(--accent-amber)' }}>
                      Manage saved manifests →
                    </NavLink>
                  )}
                </div>
              )}

              {/* Paste YAML */}
              {sourceType === 'paste' && (
                <div className="form-group">
                  <label className="form-label">Manifest YAML</label>
                  <textarea
                    className="form-input"
                    placeholder="apiVersion: content.hauler.cattle.io/v1&#10;kind: Image&#10;name: nginx:latest"
                    value={pastedYaml}
                    onChange={(e) => setPastedYaml(e.target.value)}
                    disabled={submitting}
                    style={{
                      minHeight: '200px',
                      fontFamily: 'var(--font-mono)',
                      fontSize: '0.85rem',
                      padding: '0.75rem'
                    }}
                    spellCheck="false"
                  />
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                    Paste the content of your hauler manifest file
                  </div>
                </div>
              )}

              {/* File Path Reference */}
              {sourceType === 'filepath' && (
                <div className="form-group">
                  <label className="form-label">Manifest File Paths (-f/--filename)</label>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: '0.5rem' }}>
                    Specify one or more manifest file paths. Defaults to <code>hauler-manifest.yaml</code>.
                  </div>
                  {fileList.map((file, index) => (
                    <div key={index} style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem' }}>
                      <input
                        className="form-input"
                        placeholder="/path/to/manifest.yaml"
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
              )}
            </div>

            {/* Sync Options */}
            <div className="card" style={{ marginBottom: '1rem' }}>
              <div className="card-title">
                Sync Options
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => setShowAdvanced(!showAdvanced)}
                  style={{ float: 'right', fontSize: '0.8rem' }}
                >
                  {showAdvanced ? 'Hide' : 'Show'} Advanced
                </button>
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
                  Specify the platform for images (e.g., linux/amd64, linux/arm64). Defaults to all platforms.
                </div>
              </div>

              {/* Registry Override */}
              <div className="form-group">
                <label className="form-label">Registry Override (-g)</label>
                <input
                  className="form-input"
                  placeholder="docker.io (optional)"
                  value={registry}
                  onChange={(e) => setRegistry(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Specify the registry for images that don&apos;t already define one
                </div>
              </div>

              {/* Products */}
              <div className="form-group">
                <label className="form-label">Products (--products)</label>
                <input
                  className="form-input"
                  placeholder="rancher=v2.10.1,rke2=v1.31.5+rke2r1"
                  value={products}
                  onChange={(e) => setProducts(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Fetch collections from product registry (comma-separated: name=version)
                </div>
              </div>

              {/* Product Registry */}
              <div className="form-group">
                <label className="form-label">Product Registry (--product-registry)</label>
                <input
                  className="form-input"
                  placeholder="rgcrprod.azurecr.us (default)"
                  value={productRegistry}
                  onChange={(e) => setProductRegistry(e.target.value)}
                  disabled={submitting}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Specify the product registry. Defaults to RGS Carbide Registry.
                </div>
              </div>

              {/* Advanced Options */}
              {showAdvanced && (
                <>
                  <div style={{
                    height: '1px',
                    backgroundColor: 'var(--border-color)',
                    margin: '1rem 0'
                  }}></div>

                  {/* Key for signature verification */}
                  <div className="form-group">
                    <label className="form-label">Public Key (-k)</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/public-key.pem"
                      value={key}
                      onChange={(e) => setKey(e.target.value)}
                      disabled={submitting}
                    />
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Location of public key for signature verification
                    </div>
                  </div>

                  {/* Certificate Identity */}
                  <div className="form-group">
                    <label className="form-label">Certificate Identity</label>
                    <input
                      className="form-input"
                      placeholder="identity@issuer.com"
                      value={certificateIdentity}
                      onChange={(e) => setCertificateIdentity(e.target.value)}
                      disabled={submitting}
                    />
                  </div>

                  {/* Certificate Identity Regexp */}
                  <div className="form-group">
                    <label className="form-label">Certificate Identity (Regexp)</label>
                    <input
                      className="form-input"
                      placeholder="^.*@example\\.com$"
                      value={certificateIdentityRegexp}
                      onChange={(e) => setCertificateIdentityRegexp(e.target.value)}
                      disabled={submitting}
                    />
                  </div>

                  {/* Certificate OIDC Issuer */}
                  <div className="form-group">
                    <label className="form-label">Certificate OIDC Issuer</label>
                    <input
                      className="form-input"
                      placeholder="https://issuer.example.com"
                      value={certificateOidcIssuer}
                      onChange={(e) => setCertificateOidcIssuer(e.target.value)}
                      disabled={submitting}
                    />
                  </div>

                  {/* Certificate OIDC Issuer Regexp */}
                  <div className="form-group">
                    <label className="form-label">Certificate OIDC Issuer (Regexp)</label>
                    <input
                      className="form-input"
                      placeholder="^https://issuer\\.example\\.com$"
                      value={certificateOidcIssuerRegexp}
                      onChange={(e) => setCertificateOidcIssuerRegexp(e.target.value)}
                      disabled={submitting}
                    />
                  </div>

                  {/* Certificate GitHub Workflow */}
                  <div className="form-group">
                    <label className="form-label">Certificate GitHub Workflow Repository</label>
                    <input
                      className="form-input"
                      placeholder="owner/repo"
                      value={certificateGithubWorkflow}
                      onChange={(e) => setCertificateGithubWorkflow(e.target.value)}
                      disabled={submitting}
                    />
                  </div>

                  {/* Rewrite (Experimental) */}
                  <div className="form-group">
                    <label className="form-label">Rewrite Path (Experimental)</label>
                    <input
                      className="form-input"
                      placeholder="my-registry/path"
                      value={rewrite}
                      onChange={(e) => setRewrite(e.target.value)}
                      disabled={submitting}
                    />
                    <div style={{ fontSize: '0.75rem', color: 'var(--accent-amber)', marginTop: '0.35rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                      <AlertTriangle size={14} />
                      Experimental: Rewrite artifact path to specified string
                    </div>
                  </div>

                  {/* Use Tlog Verify */}
                  <div className="form-group">
                    <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                      <input
                        type="checkbox"
                        checked={useTlogVerify}
                        onChange={(e) => setUseTlogVerify(e.target.checked)}
                        disabled={submitting}
                      />
                      <span className="form-label" style={{ margin: 0 }}>Enable Transparency Log Verification</span>
                    </label>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem', marginLeft: '1.5rem' }}>
                  Allow transparency log verification (defaults to false)
                </div>
                  </div>
                </>
              )}
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting || (
                sourceType === 'saved' && !selectedManifestId ||
                sourceType === 'paste' && !pastedYaml.trim() ||
                sourceType === 'filepath' && fileList.filter(f => f.trim()).length === 0
              )}
              style={{ fontSize: '1rem', padding: '0.75rem 1.5rem' }}
            >
              {submitting ? 'Starting Sync...' : (
                <span style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', justifyContent: 'center' }}>
                  <RefreshCw size={18} />
                  Start Sync
                </span>
              )}
            </button>
          </form>
        </div>

        {/* Help Panel */}
        <div>
          <div className="card help-panel">
            <div className="card-title">About Store Sync</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                <strong>Store Sync</strong> populates your content store from one or more manifest files.
              </p>
              <p>
                Manifests define images, charts, and files to be synced. Use saved manifests,
                paste YAML directly, or reference files by path.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Manifest Source Options</div>
            <div style={{ fontSize: '0.8rem', lineHeight: '1.7' }}>
              <p style={{ marginBottom: '0.5rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>Saved Manifest:</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: '0.75rem' }}>
                Select from previously saved manifests in the manifest library.
              </p>

              <p style={{ marginBottom: '0.5rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>Paste YAML:</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: '0.75rem' }}>
                Paste manifest YAML content directly into the form.
              </p>

              <p style={{ marginBottom: '0.5rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>File Paths:</strong>
              </p>
              <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 0, marginBottom: '0.75rem' }}>
                Reference files on the filesystem by path. Defaults to <code>hauler-manifest.yaml</code>.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">Quick Links</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              <NavLink to="/manifests" className="btn btn-sm" style={{ textAlign: 'center' }}>
                Manage Manifests
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

export default StoreSync
