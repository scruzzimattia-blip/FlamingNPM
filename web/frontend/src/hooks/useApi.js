const BASE = '/api'

async function request(path, options = {}) {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  if (res.status === 204) return null
  return res.json()
}

export const api = {
  getStats: () => request('/stats'),
  getRules: () => request('/rules'),
  createRule: (rule) => request('/rules', { method: 'POST', body: JSON.stringify(rule) }),
  updateRule: (id, rule) => request(`/rules/${id}`, { method: 'PUT', body: JSON.stringify(rule) }),
  deleteRule: (id) => request(`/rules/${id}`, { method: 'DELETE' }),
  reloadRules: () => request('/rules/reload', { method: 'POST' }),
  getLogs: (limit = 100, offset = 0) => request(`/logs?limit=${limit}&offset=${offset}`),
  getBlockedIPs: () => request('/ip-blocks'),
  blockIP: (data) => request('/ip-blocks', { method: 'POST', body: JSON.stringify(data) }),
  unblockIP: (id) => request(`/ip-blocks/${id}`, { method: 'DELETE' }),

  getProxyRoutes: () => request('/proxy-routes'),
  createProxyRoute: (rt) => request('/proxy-routes', { method: 'POST', body: JSON.stringify(rt) }),
  updateProxyRoute: (id, rt) => request(`/proxy-routes/${id}`, { method: 'PUT', body: JSON.stringify(rt) }),
  deleteProxyRoute: (id) => request(`/proxy-routes/${id}`, { method: 'DELETE' }),
}
