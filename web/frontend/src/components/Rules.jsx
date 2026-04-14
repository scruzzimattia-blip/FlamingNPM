import { useState, useEffect } from 'react'
import { Plus, Pencil, Trash2, RefreshCw, BookLock } from 'lucide-react'
import { api } from '../hooks/useApi'

export default function Rules({ events }) {
  const [rules, setRules] = useState([])
  const [showModal, setShowModal] = useState(false)
  const [editRule, setEditRule] = useState(null)

  const loadRules = () => api.getRules().then(setRules).catch(console.error)

  useEffect(() => { loadRules() }, [])

  useEffect(() => {
    const relevant = events.find(
      (e) => e.type === 'rule_created' || e.type === 'rule_updated' || e.type === 'rule_deleted'
    )
    if (relevant) loadRules()
  }, [events])

  const handleDelete = async (id) => {
    if (!confirm('Regel wirklich loeschen?')) return
    try {
      await api.deleteRule(id)
      loadRules()
    } catch (err) {
      alert('Fehler: ' + err.message)
    }
  }

  const handleToggle = async (rule) => {
    try {
      await api.updateRule(rule.id, { ...rule, enabled: !rule.enabled })
      loadRules()
    } catch (err) {
      alert('Fehler: ' + err.message)
    }
  }

  const openEdit = (rule) => {
    setEditRule(rule)
    setShowModal(true)
  }

  const openNew = () => {
    setEditRule(null)
    setShowModal(true)
  }

  return (
    <>
      <div className="page-header">
        <h2>Firewall-Regeln</h2>
        <p>Whitelist, Score-basiertes Blockieren und Sanitization</p>
      </div>

      <div className="card">
        <div className="card-header">
          <h3>{rules.length} Regeln konfiguriert</h3>
          <div className="flex-center">
            <button className="btn btn-sm btn-ghost" onClick={loadRules}>
              <RefreshCw size={14} /> Neuladen
            </button>
            <button className="btn btn-sm btn-primary" onClick={openNew}>
              <Plus size={14} /> Neue Regel
            </button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Status</th>
                <th>Name</th>
                <th>Aktion</th>
                <th>Ziel</th>
                <th>Score</th>
                <th>Pattern</th>
                <th>Beschreibung</th>
                <th className="text-right">Aktionen</th>
              </tr>
            </thead>
            <tbody>
              {rules.length === 0 ? (
                <tr>
                  <td colSpan="8" className="empty-state">
                    <BookLock size={48} />
                    <p className="mt-1">Keine Regeln konfiguriert.</p>
                  </td>
                </tr>
              ) : (
                rules.map((rule) => (
                  <tr key={rule.id}>
                    <td>
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={rule.enabled}
                          onChange={() => handleToggle(rule)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    </td>
                    <td style={{ fontWeight: 600 }}>{rule.name}</td>
                    <td>
                      <span className={`badge ${
                        rule.action === 'block' ? 'badge-danger' :
                        rule.action === 'sanitize' ? 'badge-warning' : 'badge-success'
                      }`}>
                        {rule.action === 'block' ? 'Score / Block' : rule.action === 'sanitize' ? 'Sanitize' : 'Erlauben'}
                      </span>
                    </td>
                    <td><span className="badge badge-accent">{rule.target}</span></td>
                    <td style={{ fontWeight: 600 }}>{rule.score_weight ?? 10}</td>
                    <td className="mono truncate" style={{ fontSize: '0.75rem' }}>{rule.pattern}</td>
                    <td style={{ color: 'var(--text-secondary)', maxWidth: 200 }}>{rule.description}</td>
                    <td className="text-right">
                      <div className="flex-center" style={{ justifyContent: 'flex-end' }}>
                        <button className="btn btn-icon btn-ghost" onClick={() => openEdit(rule)} title="Bearbeiten">
                          <Pencil size={14} />
                        </button>
                        <button className="btn btn-icon btn-ghost" onClick={() => handleDelete(rule.id)} title="Loeschen">
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
        <RuleModal
          rule={editRule}
          onClose={() => setShowModal(false)}
          onSaved={() => { setShowModal(false); loadRules() }}
        />
      )}
    </>
  )
}

function RuleModal({ rule, onClose, onSaved }) {
  const isEdit = !!rule
  const [form, setForm] = useState({
    name: rule?.name || '',
    pattern: rule?.pattern || '',
    target: rule?.target || 'all',
    action: rule?.action || 'block',
    score_weight: rule?.score_weight ?? 10,
    enabled: rule?.enabled ?? true,
    description: rule?.description || '',
  })
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')

    if (!form.name.trim() || !form.pattern.trim()) {
      setError('Name und Pattern sind Pflichtfelder.')
      return
    }

    try {
      new RegExp(form.pattern)
    } catch {
      setError('Ungueltiges Regex-Pattern.')
      return
    }

    setSaving(true)
    try {
      if (isEdit) {
        await api.updateRule(rule.id, form)
      } else {
        await api.createRule(form)
      }
      onSaved()
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  const update = (field, value) => setForm((f) => ({ ...f, [field]: value }))

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>{isEdit ? 'Regel bearbeiten' : 'Neue Regel erstellen'}</h3>
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Name</label>
            <input
              className="form-input"
              value={form.name}
              onChange={(e) => update('name', e.target.value)}
              placeholder="z.B. SQL Injection Filter"
            />
          </div>

          <div className="form-group">
            <label>Regex-Pattern</label>
            <input
              className="form-input mono"
              value={form.pattern}
              onChange={(e) => update('pattern', e.target.value)}
              placeholder="(?i)(union\s+select|drop\s+table)"
            />
          </div>

          <div className="form-row">
            <div className="form-group">
              <label>Ziel</label>
              <select
                className="form-input"
                value={form.target}
                onChange={(e) => update('target', e.target.value)}
              >
                <option value="all">Alles (URI + Body + Header + Param)</option>
                <option value="uri">URI</option>
                <option value="body">Body</option>
                <option value="header">Header</option>
                <option value="param">Parameter</option>
              </select>
            </div>
            <div className="form-group">
              <label>Aktion</label>
              <select
                className="form-input"
                value={form.action}
                onChange={(e) => update('action', e.target.value)}
              >
                <option value="block">Score / Block (Gewicht addieren)</option>
                <option value="allow">Erlauben (Whitelist)</option>
                <option value="sanitize">Sanitize (Treffer entfernen)</option>
              </select>
            </div>
          </div>

          {form.action === 'block' && (
            <div className="form-group">
              <label>Score-Gewicht (bei Treffer)</label>
              <input
                type="number"
                className="form-input"
                min="1"
                max="1000"
                value={form.score_weight}
                onChange={(e) => update('score_weight', parseInt(e.target.value, 10) || 10)}
              />
            </div>
          )}

          <div className="form-group">
            <label>Beschreibung</label>
            <input
              className="form-input"
              value={form.description}
              onChange={(e) => update('description', e.target.value)}
              placeholder="Kurze Beschreibung der Regel"
            />
          </div>

          <div className="form-group">
            <label className="flex-center">
              <label className="toggle">
                <input
                  type="checkbox"
                  checked={form.enabled}
                  onChange={(e) => update('enabled', e.target.checked)}
                />
                <span className="toggle-slider" />
              </label>
              <span style={{ marginLeft: '0.5rem' }}>Regel aktiv</span>
            </label>
          </div>

          {error && (
            <p style={{ color: 'var(--danger)', fontSize: '0.85rem', marginBottom: '1rem' }}>{error}</p>
          )}

          <div className="modal-actions">
            <button type="button" className="btn btn-ghost" onClick={onClose}>
              Abbrechen
            </button>
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? 'Wird gespeichert...' : isEdit ? 'Aktualisieren' : 'Erstellen'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
