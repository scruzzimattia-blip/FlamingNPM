import { useState, useEffect, useRef } from 'react'
import { ScrollText, Download, Trash2, Pause, Play } from 'lucide-react'
import { api } from '../hooks/useApi'

export default function LiveLogs({ liveLogs }) {
  const [historicLogs, setHistoricLogs] = useState([])
  const [paused, setPaused] = useState(false)
  const [filter, setFilter] = useState('')
  const logRef = useRef(null)

  useEffect(() => {
    api.getLogs(200, 0).then(setHistoricLogs).catch(console.error)
  }, [])

  const allLogs = [...liveLogs, ...historicLogs]
  const uniqueLogs = allLogs.filter(
    (log, i, self) => i === self.findIndex((l) => l.id === log.id)
  )

  const filtered = filter
    ? uniqueLogs.filter(
        (l) =>
          l.source_ip?.includes(filter) ||
          l.path?.includes(filter) ||
          l.rule_name?.toLowerCase().includes(filter.toLowerCase())
      )
    : uniqueLogs

  const displayed = paused ? filtered : filtered.slice(0, 200)

  useEffect(() => {
    if (!paused && logRef.current) {
      logRef.current.scrollTop = 0
    }
  }, [liveLogs, paused])

  const exportCSV = () => {
    const header = 'Zeitpunkt,IP,Methode,Pfad,Regel,User-Agent\n'
    const rows = displayed.map(
      (l) =>
        `"${l.timestamp}","${l.source_ip}","${l.method}","${l.path}","${l.rule_name}","${l.user_agent}"`
    )
    const blob = new Blob([header + rows.join('\n')], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `waf-logs-${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <>
      <div className="page-header">
        <h2>Live-Logs</h2>
        <p>Echtzeit-Uebersicht aller blockierten Anfragen</p>
      </div>

      <div className="card">
        <div className="card-header">
          <div className="flex-center">
            <div className="live-indicator">
              <span className="dot" />
              Live
            </div>
            <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem', marginLeft: '1rem' }}>
              {displayed.length} Eintraege
            </span>
          </div>
          <div className="flex-center">
            <input
              type="text"
              className="form-input"
              placeholder="Filtern nach IP, Pfad, Regel..."
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              style={{ width: 250 }}
            />
            <button
              className={`btn btn-sm ${paused ? 'btn-success' : 'btn-ghost'}`}
              onClick={() => setPaused(!paused)}
              title={paused ? 'Fortsetzen' : 'Pausieren'}
            >
              {paused ? <Play size={14} /> : <Pause size={14} />}
            </button>
            <button className="btn btn-sm btn-ghost" onClick={exportCSV} title="Als CSV exportieren">
              <Download size={14} />
            </button>
          </div>
        </div>

        <div ref={logRef} style={{ maxHeight: '70vh', overflowY: 'auto' }}>
          {displayed.length === 0 ? (
            <div className="empty-state">
              <ScrollText size={48} />
              <p className="mt-1">Noch keine Logs vorhanden.</p>
              <p style={{ fontSize: '0.8rem' }}>Blockierte Anfragen erscheinen hier in Echtzeit.</p>
            </div>
          ) : (
            displayed.map((log, i) => (
              <div
                key={log.id || i}
                className={`log-entry ${i < liveLogs.length && liveLogs.includes(log) ? 'new' : ''}`}
              >
                <span className="log-timestamp">
                  {new Date(log.timestamp).toLocaleTimeString('de-CH')}
                </span>
                <span className={`log-method ${log.method}`}>{log.method}</span>
                <span className="log-path">{log.path}</span>
                <span className="log-ip">{log.source_ip}</span>
                <span style={{ margin: '0 0.5rem', color: 'var(--text-muted)' }}>—</span>
                <span className="log-rule">{log.rule_name}</span>
              </div>
            ))
          )}
        </div>
      </div>
    </>
  )
}
