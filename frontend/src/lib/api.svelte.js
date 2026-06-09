const API_BASE = '/api/v1';

// Token management — initialize from localStorage for persistence across refreshes
let token = $state(typeof window !== 'undefined' ? localStorage.getItem('access_token') || '' : '');

export function setAuthToken(t) {
	token = t;
}

export function getAuthToken() {
	return token;
}

async function request(path, options = {}) {
	const headers = { 'Content-Type': 'application/json', ...options.headers };
	if (token) {
		headers['Authorization'] = `Bearer ${token}`;
	}

	const res = await fetch(`${API_BASE}${path}`, {
		...options,
		headers,
	});

	if (res.status === 401) {
		// Token expired/invalid — clear auth state and let SvelteKit redirect
		token = '';
		if (typeof window !== 'undefined') {
			localStorage.removeItem('access_token');
			localStorage.removeItem('refresh_token');
			localStorage.removeItem('user');
		}
		throw new Error('Unauthorized');
	}

	if (!res.ok) {
		const err = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(err.error || res.statusText);
	}

	const text = await res.text();
	if (!text) return null;
	const parsed = JSON.parse(text);
	// Unwrap standardized API response: {success, data, error, meta}
	if (parsed && typeof parsed === 'object') {
		if (parsed.success === false) {
			throw new Error(parsed.error || 'Request failed');
		}
		if ('data' in parsed) {
			if (parsed.meta) parsed.data._meta = parsed.meta;
			return parsed.data;
		}
	}
	return parsed;
}

export const api = {
	auth: {
		login: (email, password) =>
			request('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
		verifyTOTP: (email, token, tempToken) =>
			request('/auth/verify-totp', { method: 'POST', body: JSON.stringify({ email, token, temp_token: tempToken }) }),
		register: (email, name, password) =>
			request('/auth/register', { method: 'POST', body: JSON.stringify({ email, name, password }) }),
		me: () => request('/auth/me'),
		setupTOTP: () => request('/auth/setup-totp', { method: 'POST' }),
		verifyTOTPSetup: (token) =>
			request('/auth/verify-totp-setup', { method: 'POST', body: JSON.stringify({ token }) }),
		disableTOTP: (data) => request('/auth/disable-totp', { method: 'POST', body: JSON.stringify(data) }),
		changePassword: (data) => request('/auth/password', { method: 'PUT', body: JSON.stringify(data) }),
		updateProfile: (data) => request('/auth/profile', { method: 'PUT', body: JSON.stringify(data) }),
	},

	dashboard: {
		summary: () => request('/dashboard'),
	},

	servers: {
		list: (params = {}) => {
			const qs = new URLSearchParams();
			if (params.all) qs.set('all', 'true');
			if (params.page) qs.set('page', params.page);
			if (params.limit) qs.set('limit', params.limit);
			if (params.sort) qs.set('sort', params.sort);
			if (params.order) qs.set('order', params.order);
			if (params.status) qs.set('status', params.status);
			if (params.search) qs.set('search', params.search);
			if (params.server_group) qs.set('server_group', params.server_group);
			if (params.region) qs.set('region', params.region);
			if (params.server_type) qs.set('server_type', params.server_type);
			const q = qs.toString();
			return request(`/servers${q ? '?' + q : ''}`);
		},
		get: (id) => request(`/servers/${id}`),
		create: (data) => request('/servers', { method: 'POST', body: JSON.stringify(data) }),
		update: (id, data) => request(`/servers/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
		delete: (id) => request(`/servers/${id}`, { method: 'DELETE' }),
		bulkDelete: (ids) => request('/servers/bulk-delete', { method: 'POST', body: JSON.stringify({ ids }) }),
		testConnection: (data) => request('/servers/test', { method: 'POST', body: JSON.stringify(data) }),
		testExisting: (id) => request(`/servers/${id}/test`, { method: 'POST' }),
		metrics: (id) => request(`/servers/${id}/metrics`),
		detect: (id) => request(`/servers/${id}/detect`, { method: 'POST' }),
		containers: (id) => request(`/servers/${id}/containers`),
		containerStart: (id, container) => request(`/servers/${id}/containers/${container}/start`, { method: 'POST' }),
		containerStop: (id, container) => request(`/servers/${id}/containers/${container}/stop`, { method: 'POST' }),
		containerRestart: (id, container) => request(`/servers/${id}/containers/${container}/restart`, { method: 'POST' }),
		containerLogs: (id, container) => request(`/servers/${id}/containers/${container}/logs`),
		containerInspect: (id, container) => request(`/servers/${id}/containers/${container}/inspect`),
		containerExec: (id, container, command) => request(`/servers/${id}/containers/${container}/exec`, { method: 'POST', body: JSON.stringify({ command }) }),
		groups: () => request('/servers/groups'),
		regions: () => request('/servers/regions'),
		types: () => request('/servers/types'),
	},

	containers: {
		list: () => request('/containers'),
		byServer: () => request('/containers/by-server'),
		globalStats: () => request('/containers/stats'),
		get: (id, serverId) => request(`/containers/${id}${serverId ? '?server_id=' + serverId : ''}`),
		security: (id, serverId) => request(`/containers/${id}/security?server_id=${serverId}`),
		start: (id, serverId) => request(`/containers/${id}/start?server_id=${serverId}`, { method: 'POST' }),
		stop: (id, serverId) => request(`/containers/${id}/stop?server_id=${serverId}`, { method: 'POST' }),
		restart: (id, serverId) => request(`/containers/${id}/restart?server_id=${serverId}`, { method: 'POST' }),
		logs: (id, serverId, tail = 50) => request(`/containers/${id}/logs?server_id=${serverId}&tail=${tail}`),
		stats: (id, serverId) => request(`/containers/${id}/stats?server_id=${serverId}`),
	},

	registry: {
		config: () => request('/registry/config'),
		myCredentials: () => request('/registry/my-credentials'),
		resetMyPassword: (data) => request('/registry/my-credentials/reset-password', { method: 'POST', body: JSON.stringify(data) }),
		list: (params) => {
			const q = params ? '?' + new URLSearchParams(params).toString() : '';
			return request(`/registry/repos${q}`);
		},
		listTags: (name, params) => {
			const q = params ? '?' + new URLSearchParams(params).toString() : '';
			return request(`/registry/repos/${encodeURIComponent(name)}/tags${q}`);
		},
		detail: (name, tag) => request(`/registry/repos/${encodeURIComponent(name)}/${encodeURIComponent(tag)}`),
		delete: (name, digest) => request(`/registry/repos/${encodeURIComponent(name)}/manifests/${encodeURIComponent(digest)}`, { method: 'DELETE' }),
		deleteTag: (name, tag) => request(`/registry/repos/${encodeURIComponent(name)}/tags/${encodeURIComponent(tag)}`, { method: 'DELETE' }),
		gc: () => request('/registry/gc', { method: 'POST' }),
		users: () => request('/registry/users'),
		createUser: (data) => request('/registry/users', { method: 'POST', body: JSON.stringify(data) }),
		updateUser: (id, data) => request(`/registry/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
		deleteUser: (id) => request(`/registry/users/${id}`, { method: 'DELETE' }),
		resetPassword: (id, data) => request(`/registry/users/${id}/reset-password`, { method: 'POST', body: JSON.stringify(data) }),
		syncHtpasswd: () => request('/registry/sync-htpasswd', { method: 'POST' }),
		// Webhooks
		webhooks: {
			list: () => request('/registry/webhooks'),
			get: (id) => request(`/registry/webhooks/${id}`),
			create: (data) => request('/registry/webhooks', { method: 'POST', body: JSON.stringify(data) }),
			update: (id, data) => request(`/registry/webhooks/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
			delete: (id) => request(`/registry/webhooks/${id}`, { method: 'DELETE' }),
			test: (id) => request(`/registry/webhooks/${id}/test`, { method: 'POST' }),
			events: (params) => {
				const q = params ? '?' + new URLSearchParams(params).toString() : '';
				return request(`/registry/webhooks/events${q}`);
			},
		},
	},

	repositories: {
		list: () => request('/repositories'),
		connections: {
			list: () => request('/repositories/connections'),
			create: (data) => request('/repositories/connections', { method: 'POST', body: JSON.stringify(data) }),
			delete: (id) => request(`/repositories/connections/${id}`, { method: 'DELETE' }),
		},
		selections: {
			list: () => request('/repositories/selections'),
			save: (data) => request('/repositories/selections', { method: 'POST', body: JSON.stringify(data) }),
		},
		branches: (provider, owner, repo) => request(`/repositories/${provider}/${owner}/${repo}/branches`),
		ciStatus: (provider, owner, repo, branch) => {
			const q = branch ? `?branch=${branch}` : '';
			return request(`/repositories/${provider}/${owner}/${repo}/ci-status${q}`);
		},
		deployments: (provider, owner, repo) => request(`/repositories/${provider}/${owner}/${repo}/deployments`),
	},

	deployments: {
		list: (params) => {
			const q = params ? '?' + new URLSearchParams(params).toString() : '';
			return request(`/deployments${q}`);
		},
		create: (data) => request('/deployments', { method: 'POST', body: JSON.stringify(data) }),
		get: (id) => request(`/deployments/${id}`),
		restart: (id) => request(`/deployments/${id}/restart`, { method: 'POST' }),
		redeploy: (id) => request(`/deployments/${id}/redeploy`, { method: 'POST' }),
		rollback: (id) => request(`/deployments/${id}/rollback`, { method: 'POST' }),
		history: {
			list: () => request('/deployments/history'),
			get: (id) => request(`/deployments/${id}/history`),
		},
		environments: {
			list: () => request('/deployments/environments'),
			create: (data) => request('/deployments/environments', { method: 'POST', body: JSON.stringify(data) }),
			update: (id, data) => request(`/deployments/environments/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
			delete: (id) => request(`/deployments/environments/${id}`, { method: 'DELETE' }),
		},
	},

	sshKeys: {
		list: () => request('/ssh-keys'),
		get: (id) => request(`/ssh-keys/${id}`),
		create: (data) => request('/ssh-keys', { method: 'POST', body: JSON.stringify(data) }),
		update: (id, data) => request(`/ssh-keys/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
		delete: (id) => request(`/ssh-keys/${id}`, { method: 'DELETE' }),
	},

	compliance: {
		summary: () => request('/compliance/summary'),
		checks: () => request('/compliance/checks'),
		activeScans: () => request('/compliance/active'),
		globalHistory: (params = {}) => {
			const qs = new URLSearchParams();
			if (params.page) qs.set('page', params.page);
			if (params.limit) qs.set('limit', params.limit);
			if (params.scan_type) qs.set('scan_type', params.scan_type);
			const q = qs.toString();
			return request(`/compliance/history${q ? '?' + q : ''}`);
		},
		latest: (serverId, params = {}) => {
			const qs = new URLSearchParams();
			if (params.category) qs.set('category', params.category);
			if (params.scan_type) qs.set('scan_type', params.scan_type);
			const q = qs.toString();
			return request(`/compliance/${serverId}/latest${q ? '?' + q : ''}`);
		},
		latestCategories: (serverId, params = {}) => {
			const qs = new URLSearchParams();
			if (params.scan_type) qs.set('scan_type', params.scan_type);
			const q = qs.toString();
			return request(`/compliance/${serverId}/latest/categories${q ? '?' + q : ''}`);
		},
		categoryHistory: (serverId, category, params = {}) => {
			const qs = new URLSearchParams();
			if (params.limit) qs.set('limit', params.limit);
			const q = qs.toString();
			return request(`/compliance/${serverId}/history/categories/${category}${q ? '?' + q : ''}`);
		},
		scan: (serverId, profile = 'all') =>
			request(`/compliance/${serverId}/scan?profile=${profile}`, { method: 'POST' }),
		scanSingle: (serverId, checkId) =>
			request(`/compliance/${serverId}/scan/check/${checkId}`, { method: 'POST' }),
		scanLynis: (serverId) =>
			request(`/compliance/${serverId}/scan/lynis`, { method: 'POST' }),
		scanDocker: (serverId) =>
			request(`/compliance/${serverId}/scan?profile=cis_docker`, { method: 'POST' }),
		scanContainers: (serverId) =>
			request(`/compliance/${serverId}/scan/containers`, { method: 'POST' }),
		scanContainer: (serverId, containerId) =>
			request(`/compliance/${serverId}/scan/containers/${containerId}`, { method: 'POST' }),
		containerScanHistory: (serverId, containerName) =>
			request(`/compliance/${serverId}/containers/${encodeURIComponent(containerName)}/history`),
		history: (serverId, params = {}) => {
			const qs = new URLSearchParams();
			if (params.page) qs.set('page', params.page);
			if (params.limit) qs.set('limit', params.limit);
			if (params.scan_type) qs.set('scan_type', params.scan_type);
			const q = qs.toString();
			return request(`/compliance/${serverId}/history${q ? '?' + q : ''}`);
		},
		scanDetail: (serverId, scanId) =>
			request(`/compliance/${serverId}/history/${scanId}`),
	},

	admin: {
		users: {
			list: () => request('/admin/users'),
			get: (id) => request(`/admin/users/${id}`),
			create: (data) => request('/admin/users', { method: 'POST', body: JSON.stringify(data) }),
			update: (id, data) => request(`/admin/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
			delete: (id) => request(`/admin/users/${id}`, { method: 'DELETE' }),
			unlock: (id) => request(`/admin/users/${id}/unlock`, { method: 'POST' }),
		},
		alerts: {
			list: () => request('/admin/alerts'),
			acknowledge: (id) => request(`/admin/alerts/${id}/acknowledge`, { method: 'POST' }),
		},
		auditLog: {
			list: (params = {}) => {
				const qs = new URLSearchParams();
				if (params.page) qs.set('page', params.page);
				if (params.limit) qs.set('limit', params.limit);
				if (params.action) qs.set('action', params.action);
				if (params.entity_type) qs.set('entity_type', params.entity_type);
				if (params.user_id) qs.set('user_id', params.user_id);
				if (params.search) qs.set('search', params.search);
				if (params.start_date) qs.set('start_date', params.start_date);
				if (params.end_date) qs.set('end_date', params.end_date);
				if (params.sort) qs.set('sort', params.sort);
				if (params.order) qs.set('order', params.order);
				const q = qs.toString();
				return request(`/admin/audit-log${q ? '?' + q : ''}`);
			},
			actions: () => request('/admin/audit-log/actions'),
			entityTypes: () => request('/admin/audit-log/entity-types'),
			export: async (params = {}) => {
				const qs = new URLSearchParams();
				if (params.action) qs.set('action', params.action);
				if (params.entity_type) qs.set('entity_type', params.entity_type);
				if (params.start_date) qs.set('start_date', params.start_date);
				if (params.end_date) qs.set('end_date', params.end_date);
				if (params.search) qs.set('search', params.search);
				if (params.format) qs.set('format', params.format);
				const q = qs.toString();
				const url = `${API_BASE}/admin/audit-log/export${q ? '?' + q : ''}`;
				const res = await fetch(url, {
					headers: token ? { 'Authorization': `Bearer ${token}` } : {},
				});
				if (!res.ok) throw new Error('Export failed');
				const blob = await res.blob();
				const ext = params.format === 'csv' ? 'csv' : 'json';
				const a = document.createElement('a');
				a.href = URL.createObjectURL(blob);
				a.download = `audit-log-export.${ext}`;
				a.click();
				URL.revokeObjectURL(a.href);
			},
		},
	},

	settings: {
		complianceThresholds: () => request('/settings/compliance-thresholds'),
		updateComplianceThresholds: (data) => request('/settings/compliance-thresholds', { method: 'PUT', body: JSON.stringify(data) }),
		registration: () => request('/settings/registration'),
		updateRegistration: (data) => request('/settings/registration', { method: 'PUT', body: JSON.stringify(data) }),
	},
};
