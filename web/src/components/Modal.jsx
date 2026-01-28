import { createContext, useContext, useState, useCallback } from 'react'

const ModalContext = createContext()

export const useModal = () => {
  const context = useContext(ModalContext)
  if (!context) {
    throw new Error('useModal must be used within ModalProvider')
  }
  return context
}

export function ModalProvider({ children }) {
  const [modals, setModals] = useState([])

  const confirm = useCallback((title, message) => {
    return new Promise((resolve) => {
      const id = Date.now()
      setModals(prev => [...prev, {
        id,
        type: 'confirm',
        title,
        message,
        onConfirm: () => {
          resolve(true)
          setModals(prev => prev.filter(m => m.id !== id))
        },
        onCancel: () => {
          resolve(false)
          setModals(prev => prev.filter(m => m.id !== id))
        }
      }])
    })
  }, [])

  const alert = useCallback((title, message) => {
    return new Promise((resolve) => {
      const id = Date.now()
      setModals(prev => [...prev, {
        id,
        type: 'alert',
        title,
        message,
        onConfirm: () => {
          resolve()
          setModals(prev => prev.filter(m => m.id !== id))
        }
      }])
    })
  }, [])

  return (
    <ModalContext.Provider value={{ confirm, alert }}>
      {children}
      {modals.map(modal => (
        <Modal
          key={modal.id}
          type={modal.type}
          title={modal.title}
          message={modal.message}
          onConfirm={modal.onConfirm}
          onCancel={modal.onCancel}
        />
      ))}
    </ModalContext.Provider>
  )
}

function Modal({ type, title, message, onConfirm, onCancel }) {
  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2 className="modal-title">{title}</h2>
        </div>
        <div className="modal-body">
          <p style={{ color: 'var(--text-secondary)', lineHeight: '1.6' }}>{message}</p>
        </div>
        <div className="modal-footer">
          {type === 'alert' ? (
            <button className="btn btn-primary" onClick={onConfirm}>
              OK
            </button>
          ) : (
            <>
              <button className="btn" onClick={onCancel}>
                Cancel
              </button>
              <button className="btn btn-primary" onClick={onConfirm}>
                Confirm
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
