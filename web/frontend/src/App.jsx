import { useState, useCallback } from 'react'
import { Shield, ScrollText, BookLock, Ban, LayoutDashboard } from 'lucide-react'
import { useWebSocket } from './hooks/useWebSocket'
import Dashboard from './components/Dashboard'
import LiveLogs from './components/LiveLogs'
import Rules from './components/Rules'
import IPBlocking from './components/IPBlocking'

const NAV_ITEMS = [
  { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { id: 'logs', label: 'Live-Logs', icon: ScrollText },
  { id: 'rules', label: 'Firewall-Regeln', icon: BookLock },
  { id: 'ip-blocks', label: 'IP-Sperren', icon: Ban },
]

export default function App() {
  const [page, setPage] = useState('dashboard')
  const [liveLogs, setLiveLogs] = useState([])
  const [wsEvents, setWsEvents] = useState([])

  const handleWSMessage = useCallback((msg) => {
    if (msg.type === 'blocked_request') {
      setLiveLogs((prev) => [msg.data, ...prev].slice(0, 500))
    }
    setWsEvents((prev) => [msg, ...prev].slice(0, 100))
  }, [])

  const connected = useWebSocket(handleWSMessage)

  const renderPage = () => {
    switch (page) {
      case 'logs':
        return <LiveLogs liveLogs={liveLogs} />
      case 'rules':
        return <Rules events={wsEvents} />
      case 'ip-blocks':
        return <IPBlocking events={wsEvents} />
      default:
        return <Dashboard />
    }
  }

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="sidebar-logo">
          <h1>FlamingNPM</h1>
          <span>Web Application Firewall</span>
        </div>
        <nav className="sidebar-nav">
          {NAV_ITEMS.map((item) => (
            <a
              key={item.id}
              href="#"
              className={`sidebar-link ${page === item.id ? 'active' : ''}`}
              onClick={(e) => { e.preventDefault(); setPage(item.id) }}
            >
              <item.icon />
              {item.label}
            </a>
          ))}
        </nav>
        <div className="sidebar-status">
          <div className="flex-center">
            <span className={`status-dot ${connected ? 'online' : ''}`} />
            <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
              {connected ? 'Verbunden' : 'Verbindung wird aufgebaut...'}
            </span>
          </div>
        </div>
      </aside>
      <main className="main-content">
        {renderPage()}
      </main>
    </div>
  )
}
