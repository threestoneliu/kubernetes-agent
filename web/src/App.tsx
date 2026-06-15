import React from 'react'
import { ChatView } from './views/ChatView'
import { ClusterView } from './views/ClusterView'
import { PolicyView } from './views/PolicyView'
import { ErrorToast } from './components/ErrorToast'
import { ToastProvider, useToast } from './components/ToastProvider'

type View = 'chat' | 'clusters' | 'policies'

function Shell() {
  const [view, setView] = React.useState<View>('chat')
  const { toast, dismiss } = useToast()
  return (
    <div className="app">
      <aside className="sidebar">
        <h1>kubernetes-agent</h1>
        <button
          className={view === 'chat' ? 'active' : ''}
          onClick={() => setView('chat')}
        >
          对话
        </button>
        <button
          className={view === 'clusters' ? 'active' : ''}
          onClick={() => setView('clusters')}
        >
          集群
        </button>
        <button
          className={view === 'policies' ? 'active' : ''}
          onClick={() => setView('policies')}
        >
          策略
        </button>
      </aside>
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
      <Shell />
    </ToastProvider>
  )
}