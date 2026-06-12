import { createContext, useContext, useState, useEffect, useCallback } from 'react'

// HaulContext tracks the set of hauls and which one is "active" — the haul that
// store operations (add, sync, save, serve, ...) target by default. The active
// haul id is persisted to localStorage so it survives reloads.
const HaulContext = createContext()

const ACTIVE_HAUL_KEY = 'hauler-ui.activeHaulId'

export function useHauls() {
  const ctx = useContext(HaulContext)
  if (!ctx) {
    throw new Error('useHauls must be used within HaulProvider')
  }
  return ctx
}

export function HaulProvider({ children }) {
  const [hauls, setHauls] = useState([])
  const [activeHaulId, setActiveHaulIdState] = useState(() => {
    const stored = localStorage.getItem(ACTIVE_HAUL_KEY)
    return stored ? Number(stored) : null
  })
  const [loading, setLoading] = useState(true)

  const setActiveHaulId = useCallback((id) => {
    setActiveHaulIdState(id)
    if (id == null) {
      localStorage.removeItem(ACTIVE_HAUL_KEY)
    } else {
      localStorage.setItem(ACTIVE_HAUL_KEY, String(id))
    }
  }, [])

  const refreshHauls = useCallback(async () => {
    try {
      const res = await fetch('/api/hauls')
      if (res.ok) {
        const data = await res.json()
        const list = data.hauls || []
        setHauls(list)
        // Ensure there is always a valid active haul selected.
        setActiveHaulIdState((current) => {
          if (current && list.some((h) => h.id === current)) {
            return current
          }
          const fallback = list.length > 0 ? list[0].id : null
          if (fallback == null) {
            localStorage.removeItem(ACTIVE_HAUL_KEY)
          } else {
            localStorage.setItem(ACTIVE_HAUL_KEY, String(fallback))
          }
          return fallback
        })
      }
    } catch (err) {
      console.error('Failed to fetch hauls:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    refreshHauls()
    // Poll so item/archive counts stay current after operations complete.
    const interval = setInterval(refreshHauls, 5000)
    return () => clearInterval(interval)
  }, [refreshHauls])

  const createHaul = useCallback(async (name, description = '') => {
    const res = await fetch('/api/hauls', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description }),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(text || 'Failed to create haul')
    }
    const haul = await res.json()
    await refreshHauls()
    return haul
  }, [refreshHauls])

  const updateHaul = useCallback(async (id, fields) => {
    const res = await fetch(`/api/hauls/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(fields),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(text || 'Failed to update haul')
    }
    await refreshHauls()
    return res.json()
  }, [refreshHauls])

  const deleteHaul = useCallback(async (id) => {
    const res = await fetch(`/api/hauls/${id}`, { method: 'DELETE' })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(text || 'Failed to delete haul')
    }
    await refreshHauls()
  }, [refreshHauls])

  const activeHaul = hauls.find((h) => h.id === activeHaulId) || null

  return (
    <HaulContext.Provider
      value={{
        hauls,
        activeHaul,
        activeHaulId,
        setActiveHaulId,
        loading,
        refreshHauls,
        createHaul,
        updateHaul,
        deleteHaul,
      }}
    >
      {children}
    </HaulContext.Provider>
  )
}
