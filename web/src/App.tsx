import React, { useState } from 'react'
import { ChatView } from './views/ChatView'
import { ClusterView } from './views/ClusterView'
import { PolicyView } from './views/PolicyView'
import { ErrorToast } from './components/ErrorToast'
import { ToastProvider, useToast } from './components/ToastProvider'
import { ThemeProvider, useTheme } from './contexts/ThemeContext'

type View = 'chat' | 'clusters' | 'policies'

function Shell() {
  const [view, setView] = useState<View>('chat')
  const { theme, toggle } = useTheme()
  const { toast, dismiss } = useToast()
  return (
    <div className="app">
      {/* Left nav — 60px icon nav */}
      <nav className="nav">
        <button
          className={`nav-item ${view === 'chat' ? 'active' : ''}`}
          onClick={() => setView('chat')}
          title="对话"
        >
          💬
        </button>
        <button
          className={`nav-item ${view === 'clusters' ? 'active' : ''}`}
          onClick={() => setView('clusters')}
          title="集群"
        >
          ☸
        </button>
        <button
          className={`nav-item ${view === 'policies' ? 'active' : ''}`}
          onClick={() => setView('policies')}
          title="策略"
        >
          🛡
        </button>

        <div style={{ flex: 1 }} />

        <button className="nav-item" onClick={toggle} title="切换主题">
          {theme === 'dark' ? '🌙' : '☀️'}
        </button>
        <button className="nav-item" title="设置">
          ⚙
        </button>
      </nav>

      {/* Right — main content, always full width */}
      <main className="main">
        {view === 'chat' && <ChatView />}
        {view === 'clusters' && <ClusterView />}
        {view === 'policies' && <PolicyView />}
      </main>

      {toast && <ErrorToast message={toast} onDismiss={dismiss} />}
    </div>
  )
}

export function App() {
  return (
    <ToastProvider>
      <ThemeProvider>
        <Shell />
      </ThemeProvider>
    </ToastProvider>
  )
}
