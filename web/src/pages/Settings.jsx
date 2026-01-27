import { useState, useEffect } from 'react'
import './Settings.css'

function Settings() {
  const [config, setConfig] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    fetch('/api/config')
      .then(res => res.json())
      .then(data => {
        setConfig(data)
        setLoading(false)
      })
      .catch(err => {
        setError(err.message)
        setLoading(false)
      })
  }, [])

  if (loading) {
    return <div className="settings-page loading">Loading configuration...</div>
  }

  if (error) {
    return <div className="settings-page error">Error loading configuration: {error}</div>
  }

  return (
    <div className="settings-page">
      <h1>Settings</h1>
      <p className="subtitle">View current hauler directory configuration</p>

      <section className="settings-section">
        <h2>Directory Paths</h2>
        <div className="setting-item">
          <label>Hauler Dir</label>
          <code className="setting-value">{config.haulerDir}</code>
          <span className="env-var">Environment: {config.haulerDirEnv}</span>
        </div>
        <div className="setting-item">
          <label>Hauler Store Dir</label>
          <code className="setting-value">{config.haulerStoreDir}</code>
          <span className="env-var">Environment: {config.haulerStoreEnv}</span>
        </div>
        <div className="setting-item">
          <label>Hauler Temp Dir</label>
          <code className="setting-value">{config.haulerTempDir}</code>
          <span className="env-var">Environment: {config.haulerTempEnv}</span>
        </div>
      </section>

      <section className="settings-section">
        <h2>Docker Authentication</h2>
        <div className="setting-item">
          <label>Docker Config Path</label>
          <code className="setting-value">{config.dockerAuthPath}</code>
          <span className="env-var">Environment: {config.dockerConfigEnv}</span>
        </div>
        <p className="info-text">
          <strong>Note:</strong> When you run <code>hauler login</code>, credentials are stored
          in <code>~/.docker/config.json</code> which resolves to the path shown above
          (inside the container at <code>/data/.docker/config.json</code>).
          This file persists to your host via the <code>/data</code> volume mount.
        </p>
      </section>

      <section className="settings-section">
        <h2>Persistence Information</h2>
        <p className="info-text">
          All data is persisted to the <code>/data</code> volume inside the container.
          Ensure you mount this volume when running the container:
        </p>
        <pre className="code-example">
{`docker run -v ./data:/data -p 8080:8080 hauler-ui:latest`}
        </pre>
      </section>
    </div>
  )
}

export default Settings
