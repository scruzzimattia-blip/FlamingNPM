import { useState, useEffect } from 'react'
import { ShieldAlert, ShieldCheck, ListFilter, Ban, Activity } from 'lucide-react'
import { api } from '../hooks/useApi'

export default function Dashboard() {
  const [stats, setStats] = useState(null)
  const [recentLogs, setRecentLogs] = useState([])

  useEffect(() => {
    const load = async () => {
      try {
        const [s, logs] = await Promise.all([api.getStats(), api.getLogs(10, 0)])
        setStats(s)
        setRecentLogs(logs)
      } catch (err) {
        console.error('Dashboard laden fehlgeschlagen:', err)
      }
    }
    load()
    const interval = setInterval(load, 10000)
    return () => clearInterval(interval)
  }, [])

  if (!stats) {
    return (
      <div className="empty-state">
        <p>Dashboard wird geladen...</p>
      </div>
    )
  }

  return (
    <>
      <div className="page-header">
        <h2>Dashboard</h2>
        <p>Uebersicht der WAF-Aktivitaeten</p>
      </div>

      <div className="stats-grid">
        <div className="stat-card">
          <div className="label">Blockiert (Gesamt)</div>
          <div className="value danger">{stats.total_blocked.toLocaleString()}</div>
        </div>
        <div className="stat-card">
          <div className="label">Blockiert (Heute)</div>
          <div className="value warning">{stats.blocked_today.toLocaleString()}</div>
        </div>
        <div className="stat-card">
          <div className="label">Aktive Regeln</div>
          <div className="value accent">{stats.active_rules}</div>
        </div>
        <div className="stat-card">
          <div className="label">Gesperrte IPs</div>
          <div className="value danger">{stats.blocked_ips}</div>
        </div>
        <div className="stat-card">
          <div className="label">Blocks / Minute</div>
          <div className="value success">{stats.requests_per_min}</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <h3>Letzte blockierte Anfragen</h3>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Zeitpunkt</th>
                <th>IP</th>
                <th>Methode</th>
                <th>Pfad</th>
                <th>Regel</th>
              </tr>
            </thead>
            <tbody>
              {recentLogs.length === 0 ? (
                <tr>
                  <td colSpan="5" className="empty-state">
                    Noch keine blockierten Anfragen vorhanden.
                  </td>
                </tr>
              ) : (
                recentLogs.map((log) => (
                  <tr key={log.id}>
                    <td className="mono" style={{ fontSize: '0.8rem', whiteSpace: 'nowrap' }}>
                      {new Date(log.timestamp).toLocaleString('de-CH')}
                    </td>
                    <td className="mono">{log.source_ip}</td>
                    <td>
                      <span className={`badge badge-${log.method === 'GET' ? 'success' : 'warning'}`}>
                        {log.method}
                      </span>
                    </td>
                    <td className="mono truncate">{log.path}</td>
                    <td><span className="badge badge-danger">{log.rule_name}</span></td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </>
  )
}
