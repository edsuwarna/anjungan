const API_BASE = '/api/v1';

async function request(endpoint, options = {}) {
	const token = localStorage.getItem('access_token');
	const headers = { 'Content-Type': 'application/json', ...options.headers };
	if (token) headers['Authorization'] = `Bearer ${token}`;

	const res = await fetch(`${API_BASE}${endpoint}`, { ...options, headers });
	const json = await res.json();

	if (!json.success) throw new Error(json.error || 'request failed');
	return json.data;
}

export const api = {
	get: (endpoint) => request(endpoint),
	post: (endpoint, data) => request(endpoint, { method: 'POST', body: JSON.stringify(data) }),
	put: (endpoint, data) => request(endpoint, { method: 'PUT', body: JSON.stringify(data) }),
	delete: (endpoint) => request(endpoint, { method: 'DELETE' }),

	auth: {
		login: (email, password, totpCode) =>
			request('/auth/login', {
				method: 'POST',
				body: JSON.stringify({ email, password, totp_code: totpCode })
			}),
		register: (email, name, password) =>
			request('/auth/register', {
				method: 'POST',
				body: JSON.stringify({ email, name, password })
			}),
		me: () => request('/auth/me'),
		logout: () => request('/auth/logout', { method: 'POST' })
	},

	dashboard: {
		summary: () => request('/dashboard')
	},

	servers: {
		list: () => request('/servers'),
		get: (id) => request(`/servers/${id}`),
		delete: (id) => request(`/servers/${id}`, { method: 'DELETE' })
	},

	containers: {
		list: () => request('/containers'),
		stats: () => request('/containers/stats')
	},

	registry: {
		list: () => request('/registry')
	},

	repositories: {
		list: () => request('/repositories')
	},

	admin: {
		users: () => request('/admin/users')
	}
};
