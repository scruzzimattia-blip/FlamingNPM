import { useState, useEffect } from 'react'
import { Plus, Pencil, Trash2, RefreshCw, Server } from 'lucide-react'
import { api } from '../hooks/useApi'

export default function ProxyRoutes({ events }) {
  const [routes, setRoutes] = useState([])
  const [showModal, setShowModal] = useState(false)
  const [editRoute, setEditRoute] = useState(null)

  const load = () => api.getProxyRoutes().then(setRoutes).catch(console.error)

  useEffect(() => { load() }, [])

  useEffect(() => {
    const t = ['proxy_route_created', 'proxy_route_updated', 'proxy_route_deleted']
    if (events.find((e) => t.includes(e.type))) load()
  }, [events])

  const handleDelete = async (id) => {
    if (!confirm('Route wirklich loeschen?')) return
    try {
      await api.deleteProxyRoute(id)
      load()
    } catch (err) {
      alert('Fehler: ' + err.message)
    }
  }

  const toggle = async (rt) => {
    try {
      await api.updateProxyRoute(rt.id, { ...rt, enabled: !rt.enabled })
      load()
    } catch (err) {
      alert('Fehler: ' + err.message)
    }
  }

  return (
    <>
      <div className="page-header">
        <h2>Proxy-Routen</h2>
        <p>Host-Header zu Backend-URLs zuordnen (Fallback: BACKEND_URL)</p>
      </div>

      <div className="card">
        <div className="card-header">
          <h3>{routes.length} Eintraege</h3>
          <div className="flex-center">
            <button type="button" className="btn btn-sm btn-ghost" onClick={load}>
              <RefreshCw size={14} /> Neuladen
            </button>
            <button type="button" className="btn btn-sm btn-primary" onClick={() => { setEditRoute(null); setShowModal(true) }}>
              <Plus size={14} /> Neue Route
            </button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Aktiv</th>
                <th>Host</th>
                <th>Backend</th>
                <th>Pfad-Prefix</th>
                <th>Prioritaet</th>
                <th className="text-right">Aktionen</th>
              </tr>
            </thead>
            <tbody>
              {routes.length === 0 ? (
                <tr>
                  <td colSpan="6" className="empty-state">
                    <Server size={48} />
                    <p className="mt-1">Keine Routen — es wird nur der Standard-Upstream verwendet.</p>
                  </td>
                </tr>
              ) : (
                routes.map((rt) => (
                  <tr key={rt.id}>
                    <td>
                      <label className="toggle">
                        <input type="checkbox" checked={rt.enabled} onChange={() => toggle(rt)} />
                        <span className="toggle-slider" />
                      </label>
                    </td>
                    <td className="mono" style={{ fontWeight: 600 }}>{rt.host}</td>
                    <td className="mono truncate" style={{ fontSize: '0.8rem' }}>{rt.backend_url}</td>
                    <td className="mono">{rt.path_prefix || '—'}</td>
                    <td>{rt.priority}</td>
                    <td className="text-right">
                      <div className="flex-center" style={{ justifyContent: 'flex-end' }}>
                        <button type="button" className="btn btn-icon btn-ghost" onClick={() => { setEditRoute(rt); setShowModal(true) }} title="Bearbeiten">
                          <Pencil size={14} />
                        </button>
                        <button type="button" className="btn btn-icon btn-ghost" onClick={() => handleDelete(rt.id)} title="Loeschen">
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {showModal && (
        <RouteModal
          route={editRoute}
          onClose={() => setShowModal(false)}
          onSaved={() => { setShowModal(false); load() }}
        />
      )}
    </>
  )
}

function RouteModal({ route, onClose, onSaved }) {
  const isEdit = !!route
  const [form, setForm] = useState({
    host: route?.host || '',
    backend_url: route?.backend_url || 'http://',
    path_prefix: route?.path_prefix || '',
    enabled: route?.enabled ?? true,
    priority: route?.priority ?? 0,
  })
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const update = (k, v) => setForm((f) => ({ ...f, [k]: v }))

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    if (!form.host.trim()) {
      setError('Host ist Pflicht.')
      return
    }
    if (!form.backend_url.trim()) {
      setError('Backend-URL ist Pflicht.')
      return
    }
    setSaving(true)
    try {
      if (isEdit) {
        await api.updateProxyRoute(route.id, form)
      } else {
        await api.createProxyRoute(form)
      }
      onSaved()
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(ev) => ev.stopPropagation()}>
        <h3>{isEdit ? 'Route bearbeiten' : 'Neue Proxy-Route'}</h3>
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Host (Host-Header)</label>
            <input className="form-input mono" value={form.host} onChange={(e) => update('host', e.target.value)} placeholder="app.example.com" />
          </div>
          <div className="form-group">
            <label>Backend-URL</label>
            <input className="form-input mono" value={form.backend_url} onChange={(e) => update('backend_url', e.target.value)} placeholder="http://mein-service:8080" />
          </div>
          <div className="form-group">
            <label>Pfad-Prefix (optional, wird beim Upstream entfernt)</label>
            <input className="form-input mono" value={form.path_prefix} onChange={(e) => update('path_prefix', e.target.value)} placeholder="/api" />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label>Prioritaet</label>
              <input type="number" className="form-input" value={form.priority} onChange={(e) => update('priority', parseInt(e.target.value, 10) || 0)} />
            </div>
            <div className="form-group">
              <label className="flex-center" style={{ marginTop: '1.5rem' }}>
                <label className="toggle">
                  <input type="checkbox" checked={form.enabled} onChange={(e) => update('enabled', e.target.checked)} />
                  <span className="toggle-slider" />
                </label>
                <span style={{ marginLeft: '0.5rem' }}>Aktiv</span>
              </label>
            </div>
          </div>
          {error && <p style={{ color: 'var(--danger)', fontSize: '0.85rem' }}>{error}</p>}
          <div className="modal-actions">
            <button type="button" className="btn btn-ghost" onClick={onClose}>Abbrechen</button>
            <button type="submit" className="btn btn-primary" disabled={saving}>{saving ? 'Speichern...' : 'Speichern'}</button>
          </div>
        </form>
      </div>
    </div>
  )
}
