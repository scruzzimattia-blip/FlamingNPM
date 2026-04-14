import { useState, useEffect } from 'react'
import { Ban, Plus, Trash2, Clock, Shield } from 'lucide-react'
import { api } from '../hooks/useApi'

export default function IPBlocking({ events }) {
  const [blockedIPs, setBlockedIPs] = useState([])
  const [showModal, setShowModal] = useState(false)

  const loadIPs = () => api.getBlockedIPs().then(setBlockedIPs).catch(console.error)

  useEffect(() => { loadIPs() }, [])

  useEffect(() => {
    const relevant = events.find(
      (e) => e.type === 'ip_blocked' || e.type === 'ip_unblocked'
    )
    if (relevant) loadIPs()
  }, [events])

  const handleUnblock = async (id) => {
    if (!confirm('IP-Sperre wirklich aufheben?')) return
    try {
      await api.unblockIP(id)
      loadIPs()
    } catch (err) {
      alert('Fehler: ' + err.message)
    }
  }

  return (
    <>
      <div className="page-header">
        <h2>IP-Sperren</h2>
        <p>IP-Adressen manuell sperren und verwalten</p>
      </div>

      <div className="card">
        <div className="card-header">
          <h3>{blockedIPs.length} IP-Adressen gesperrt</h3>
          <button className="btn btn-sm btn-danger" onClick={() => setShowModal(true)}>
            <Plus size={14} /> IP sperren
          </button>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>IP-Adresse</th>
                <th>Grund</th>
                <th>Ablauf</th>
                <th>Erstellt</th>
                <th className="text-right">Aktion</th>
              </tr>
            </thead>
            <tbody>
              {blockedIPs.length === 0 ? (
                <tr>
                  <td colSpan="5" className="empty-state">
                    <Shield size={48} />
                    <p className="mt-1">Keine IP-Adressen gesperrt.</p>
                  </td>
                </tr>
              ) : (
                blockedIPs.map((block) => (
                  <tr key={block.id}>
                    <td className="mono" style={{ fontWeight: 600 }}>{block.ip}</td>
                    <td style={{ color: 'var(--text-secondary)' }}>{block.reason || '—'}</td>
                    <td>
                      {block.expires_at ? (
                        <span className="flex-center" style={{ color: 'var(--warning)' }}>
                          <Clock size={14} />
                          {new Date(block.expires_at).toLocaleString('de-CH')}
                        </span>
                      ) : (
                        <span className="badge badge-danger">Permanent</span>
                      )}
                    </td>
                    <td className="mono" style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                      {new Date(block.created_at).toLocaleString('de-CH')}
                    </td>
                    <td className="text-right">
                      <button
                        className="btn btn-sm btn-ghost"
                        onClick={() => handleUnblock(block.id)}
                        title="Sperre aufheben"
                      >
                        <Trash2 size={14} /> Entsperren
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {showModal && (
        <BlockIPModal
          onClose={() => setShowModal(false)}
          onSaved={() => { setShowModal(false); loadIPs() }}
        />
      )}
    </>
  )
}

function BlockIPModal({ onClose, onSaved }) {
  const [ip, setIP] = useState('')
  const [reason, setReason] = useState('')
  const [duration, setDuration] = useState('permanent')
  const [customMinutes, setCustomMinutes] = useState(60)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')

    if (!ip.trim()) {
      setError('IP-Adresse ist ein Pflichtfeld.')
      return
    }

    const ipv4 = /^(\d{1,3}\.){3}\d{1,3}$/
    const ipv6 = /^[0-9a-fA-F:]+$/
    if (!ipv4.test(ip.trim()) && !ipv6.test(ip.trim())) {
      setError('Ungueltige IP-Adresse.')
      return
    }

    let expiresAt = null
    if (duration !== 'permanent') {
      const minutes = duration === 'custom' ? customMinutes : parseInt(duration)
      const d = new Date()
      d.setMinutes(d.getMinutes() + minutes)
      expiresAt = d.toISOString()
    }

    setSaving(true)
    try {
      await api.blockIP({ ip: ip.trim(), reason, expires_at: expiresAt })
      onSaved()
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>IP-Adresse sperren</h3>
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>IP-Adresse</label>
            <input
              className="form-input mono"
              value={ip}
              onChange={(e) => setIP(e.target.value)}
              placeholder="192.168.1.100"
            />
          </div>

          <div className="form-group">
            <label>Grund (optional)</label>
            <input
              className="form-input"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="z.B. Wiederholte Brute-Force-Versuche"
            />
          </div>

          <div className="form-group">
            <label>Dauer</label>
            <select
              className="form-input"
              value={duration}
              onChange={(e) => setDuration(e.target.value)}
            >
              <option value="permanent">Permanent</option>
              <option value="15">15 Minuten</option>
              <option value="60">1 Stunde</option>
              <option value="360">6 Stunden</option>
              <option value="1440">24 Stunden</option>
              <option value="10080">7 Tage</option>
              <option value="custom">Benutzerdefiniert</option>
            </select>
          </div>

          {duration === 'custom' && (
            <div className="form-group">
              <label>Dauer in Minuten</label>
              <input
                type="number"
                className="form-input"
                value={customMinutes}
                onChange={(e) => setCustomMinutes(parseInt(e.target.value) || 0)}
                min="1"
              />
            </div>
          )}

          {error && (
            <p style={{ color: 'var(--danger)', fontSize: '0.85rem', marginBottom: '1rem' }}>{error}</p>
          )}

          <div className="modal-actions">
            <button type="button" className="btn btn-ghost" onClick={onClose}>
              Abbrechen
            </button>
            <button type="submit" className="btn btn-danger" disabled={saving}>
              {saving ? 'Wird gesperrt...' : 'IP sperren'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
