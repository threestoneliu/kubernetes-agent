import React, { useState } from 'react'
import { ChatView } from './views/ChatView'
import { ClusterView } from './views/ClusterView'
import { PolicyView } from './views/PolicyView'
import { ScheduledTasksView } from './views/ScheduledTasksView'
import { ErrorToast } from './components/ErrorToast'
import { ToastProvider, useToast } from './components/ToastProvider'
import { ThemeProvider, useTheme } from './contexts/ThemeContext'

type View = 'chat' | 'clusters' | 'policies' | 'tasks'

function Shell() {
  const [view, setView] = useState<View>('chat')
  const { theme, toggle } = useTheme()
  const { toast, dismiss } = useToast()
  return (
    <div className="app">
      <header className="header-bar">
        <div className="header-logo">
          <span>🤖</span>
          <span>Kubernetes Agent</span>
        </div>
        <nav className="header-nav">
          <button
            className={`nav-tab ${view === 'chat' ? 'active' : ''}`}
            onClick={() => setView('chat')}
          >
            对话
          </button>
          <button
            className={`nav-tab ${view === 'clusters' ? 'active' : ''}`}
            onClick={() => setView('clusters')}
          >
            集群
          </button>
          <button
            className={`nav-tab ${view === 'policies' ? 'active' : ''}`}
            onClick={() => setView('policies')}
          >
            策略
          </button>
          <button
            className={`nav-tab ${view === 'tasks' ? 'active' : ''}`}
            onClick={() => setView('tasks')}
          >
            定时任务
          </button>
        </nav>
        <div className="header-actions">
          <button className="icon-btn" onClick={toggle} title="切换主题">
            {theme === 'dark' ? '🌙' : '☀️'}
          </button>
          <button className="icon-btn" title="设置">
            ⚙
          </button>
        </div>
      </header>
      <div className="app-body">
        <main className="main">
          {view === 'chat' && <ChatView />}
          {view === 'clusters' && <ClusterView />}
          {view === 'policies' && <PolicyView />}
          {view === 'tasks' && <ScheduledTasksView />}
        </main>
      </div>
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
