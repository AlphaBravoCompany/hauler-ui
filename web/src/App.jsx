import { useState, useEffect } from 'react'
import { HashRouter as Router, Routes, Route, NavLink } from 'react-router-dom'
import './App.css'
import Settings from './pages/Settings'

function Dashboard() {
  const [health, setHealth] = useState(null)

  useEffect(() => {
    fetch('/healthz')
      .then(res => res.json())
      .then(data => setHealth(data))
      .catch(() => setHealth({ status: 'error' }))
  }, [])

  return (
    <div className="dashboard">
      <h2>Dashboard</h2>
      {health && <p className="health-status">Backend: {health.status}</p>}
      <p>Welcome to Hauler UI. Use the navigation to manage your hauler store.</p>
    </div>
  )
}

function App() {
  return (
    <Router>
      <div className="App">
        <nav className="navbar">
          <div className="nav-brand">Hauler UI</div>
          <div className="nav-links">
            <NavLink to="/" className="nav-link" end>Dashboard</NavLink>
            <NavLink to="/settings" className="nav-link">Settings</NavLink>
          </div>
        </nav>
        <main className="main-content">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </main>
      </div>
    </Router>
  )
}

export default App
