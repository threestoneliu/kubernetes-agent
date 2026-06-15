import React from 'react'

type ToastCtx = {
  toast: string | null
  show: (message: string) => void
  dismiss: () => void
}

const ToastContext = React.createContext<ToastCtx | null>(null)

const AUTO_DISMISS_MS = 5000

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toast, setToast] = React.useState<string | null>(null)
  const timer = React.useRef<ReturnType<typeof setTimeout> | null>(null)

  const dismiss = React.useCallback(() => {
    if (timer.current) {
      clearTimeout(timer.current)
      timer.current = null
    }
    setToast(null)
  }, [])

  const show = React.useCallback(
    (message: string) => {
      if (timer.current) clearTimeout(timer.current)
      setToast(message)
      timer.current = setTimeout(() => {
        setToast(null)
        timer.current = null
      }, AUTO_DISMISS_MS)
    },
    []
  )

  // Drop any pending timer on unmount so we don't setState on an
  // unmounted component.
  React.useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current)
  }, [])

  const value = React.useMemo(() => ({ toast, show, dismiss }), [toast, show, dismiss])
  return <ToastContext.Provider value={value}>{children}</ToastContext.Provider>
}

export function useToast(): ToastCtx {
  const ctx = React.useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used inside <ToastProvider>')
  return ctx
}