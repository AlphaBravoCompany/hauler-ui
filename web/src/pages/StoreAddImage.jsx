import { useState } from 'react'
import { useNavigate, NavLink } from 'react-router-dom'
import { useJobs } from '../App.jsx'

function StoreAddImage() {
  const { fetchJobs } = useJobs()
  const navigate = useNavigate()

  const [imageRef, setImageRef] = useState('')
  const [platform, setPlatform] = useState('')
  const [key, setKey] = useState('')
  const [certificateIdentity, setCertificateIdentity] = useState('')
  const [certificateIdentityRegexp, setCertificateIdentityRegexp] = useState('')
  const [certificateOidcIssuer, setCertificateOidcIssuer] = useState('')
  const [certificateOidcIssuerRegexp, setCertificateOidcIssuerRegexp] = useState('')
  const [certificateGithubWorkflow, setCertificateGithubWorkflow] = useState('')
  const [rewrite, setRewrite] = useState('')
  const [useTlogVerify, setUseTlogVerify] = useState(false)

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [showAdvanced, setShowAdvanced] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError(null)
    setLoading(true)

    try {
      const res = await fetch('/api/store/add-image', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          imageRef,
          platform: platform || undefined,
          key: key || undefined,
          certificateIdentity: certificateIdentity || undefined,
          certificateIdentityRegexp: certificateIdentityRegexp || undefined,
          certificateOidcIssuer: certificateOidcIssuer || undefined,
          certificateOidcIssuerRegexp: certificateOidcIssuerRegexp || undefined,
          certificateGithubWorkflow: certificateGithubWorkflow || undefined,
          rewrite: rewrite || undefined,
          useTlogVerify
        })
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data.message || 'Add image request failed')
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
          <h1 className="page-title">Store Add Image</h1>
          <p className="page-subtitle">Add container images to the hauler store</p>
        </div>
        <NavLink to="/store" className="btn">← Back to Store</NavLink>
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
            <div className="card-title">Image Reference</div>
            <form onSubmit={handleSubmit}>
              <div className="form-group">
                <label className="form-label">Image Reference *</label>
                <input
                  className="form-input"
                  placeholder="busybox or docker.io/library/busybox:latest"
                  value={imageRef}
                  onChange={(e) => setImageRef(e.target.value)}
                  disabled={loading}
                  required
                  autoFocus
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  The container image reference to store (e.g., busybox, repo:tag, or digest)
                </div>
              </div>

              <div className="form-group">
                <label className="form-label">Platform (Optional)</label>
                <input
                  className="form-input"
                  placeholder="linux/amd64"
                  value={platform}
                  onChange={(e) => setPlatform(e.target.value)}
                  disabled={loading}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Specify the platform of the image (defaults to all platforms)
                </div>
              </div>

              <div className="form-group">
                <label className="form-label">Rewrite Path (Optional)</label>
                <input
                  className="form-input"
                  placeholder="custom-path/busybox:latest"
                  value={rewrite}
                  onChange={(e) => setRewrite(e.target.value)}
                  disabled={loading}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Rewrite the artifact path to the specified string
                </div>
              </div>

              <button
                type="button"
                className="btn btn-sm"
                onClick={() => setShowAdvanced(!showAdvanced)}
                style={{ marginBottom: '1rem' }}
              >
                {showAdvanced ? '▼' : '▶'} {showAdvanced ? 'Hide' : 'Show'} Advanced Options
              </button>

              {showAdvanced && (
                <div style={{
                  padding: '1rem',
                  backgroundColor: 'var(--bg-tertiary)',
                  border: '1px dashed var(--border-color)',
                  borderRadius: '2px',
                  marginBottom: '1rem'
                }}>
                  <div className="card-title" style={{ border: 'none', paddingBottom: '0' }}>Signature Verification</div>

                  <div className="form-group">
                    <label className="form-label">Key Path (Optional)</label>
                    <input
                      className="form-input"
                      placeholder="/path/to/key.pub"
                      value={key}
                      onChange={(e) => setKey(e.target.value)}
                      disabled={loading}
                    />
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Location of public key to use for signature verification
                    </div>
                  </div>

                  <div className="form-group">
                    <label className="form-label">Certificate Identity (Optional)</label>
                    <input
                      className="form-input"
                      placeholder="expected-identity"
                      value={certificateIdentity}
                      onChange={(e) => setCertificateIdentity(e.target.value)}
                      disabled={loading}
                    />
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Cosign certificate identity for keyless verification
                    </div>
                  </div>

                  <div className="form-group">
                    <label className="form-label">Certificate Identity Regexp (Optional)</label>
                    <input
                      className="form-input"
                      placeholder="^.*@example.com$"
                      value={certificateIdentityRegexp}
                      onChange={(e) => setCertificateIdentityRegexp(e.target.value)}
                      disabled={loading}
                    />
                  </div>

                  <div className="form-group">
                    <label className="form-label">Certificate OIDC Issuer (Optional)</label>
                    <input
                      className="form-input"
                      placeholder="https://issuer.example.com"
                      value={certificateOidcIssuer}
                      onChange={(e) => setCertificateOidcIssuer(e.target.value)}
                      disabled={loading}
                    />
                  </div>

                  <div className="form-group">
                    <label className="form-label">Certificate OIDC Issuer Regexp (Optional)</label>
                    <input
                      className="form-input"
                      placeholder="^https://issuer\\.example\\.com$"
                      value={certificateOidcIssuerRegexp}
                      onChange={(e) => setCertificateOidcIssuerRegexp(e.target.value)}
                      disabled={loading}
                    />
                  </div>

                  <div className="form-group">
                    <label className="form-label">GitHub Workflow Repository (Optional)</label>
                    <input
                      className="form-input"
                      placeholder="owner/repo"
                      value={certificateGithubWorkflow}
                      onChange={(e) => setCertificateGithubWorkflow(e.target.value)}
                      disabled={loading}
                    />
                  </div>

                  <div className="form-group">
                    <label className="form-label" style={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}>
                      <input
                        type="checkbox"
                        checked={useTlogVerify}
                        onChange={(e) => setUseTlogVerify(e.target.checked)}
                        disabled={loading}
                        style={{ marginRight: '0.5rem' }}
                      />
                      Use Transparency Log Verification
                    </label>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                      Allow transparency log verification (defaults to false)
                    </div>
                  </div>
                </div>
              )}

              <button type="submit" className="btn btn-primary" disabled={loading || !imageRef}>
                {loading ? 'Adding Image...' : 'Add Image to Store'}
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
                <strong style={{ color: 'var(--accent-amber)' }}>Simple image:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                busybox
              </code>

              <p style={{ marginBottom: '0.75rem', marginTop: '1rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>Repository with tag:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                library/busybox:stable
              </code>

              <p style={{ marginBottom: '0.75rem', marginTop: '1rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>Full registry with platform:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                ghcr.io/hauler-dev/hauler-debug:v1.2.0<br />
                Platform: linux/amd64
              </code>

              <p style={{ marginBottom: '0.75rem', marginTop: '1rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>By digest:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                gcr.io/distroless/base@sha256:7fa7...
              </code>

              <p style={{ marginBottom: '0.75rem', marginTop: '1rem' }}>
                <strong style={{ color: 'var(--accent-amber)' }}>With signature verification:</strong>
              </p>
              <code style={{ display: 'block', padding: '0.4rem', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', fontSize: '0.75rem' }}>
                rgcrprod.azurecr.us/rancher/rke2-runtime:v1.31.5-rke2r1<br />
                Key: carbide-key.pub<br />
                Platform: linux/amd64
              </code>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">About Platform</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                If no platform is specified, hauler will fetch <strong>all available platforms</strong> for the image.
              </p>
              <p>
                Use the platform flag when you only need a specific architecture, such as <code>linux/amd64</code> or <code>linux/arm64</code>.
              </p>
            </div>
          </div>

          <div className="card help-panel" style={{ marginTop: '1rem' }}>
            <div className="card-title">About Signature Verification</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: '1.6' }}>
              <p style={{ marginTop: 0 }}>
                Hauler supports <strong>cosign signature verification</strong> for container images.
              </p>
              <p>
                Use <strong>key-based verification</strong> by providing a public key file, or use <strong>keyless verification</strong> with certificate identity options.
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default StoreAddImage
