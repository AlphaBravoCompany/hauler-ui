import { useState, useEffect } from 'react'
import './App.css'

function App() {
  const [health, setHealth] = useState(null)

  useEffect(() => {
    fetch('/healthz')
      .then(res => res.json())
      .then(data => setHealth(data))
      .catch(() => setHealth({ status: 'error' }))
  }, [])

  return (
    <div className="App">
      <header className="App-header">
        <h1>Hauler UI</h1>
        <p>Web interface for Rancher Government Hauler CLI</p>
        {health && <p className="health-status">Backend: {health.status}</p>}
      </header>
    </div>
  )
}

export default App
