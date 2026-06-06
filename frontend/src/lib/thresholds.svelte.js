// Centralized compliance score thresholds.
// Fetches from backend settings on load, falls back to hardcoded defaults.

import { api } from '$lib/api.svelte.js';

// Hardcoded defaults — matches backend DefaultComplianceThresholds()
const DEFAULTS = { compliant: 90, warning: 70 };

let _thresholds = $state({ ...DEFAULTS });
let _loaded = $state(false);
let _loading = false;

/**
 * Fetch thresholds from the backend once.
 * Safe to call multiple times — only fetches once.
 */
export async function loadThresholds() {
	if (_loaded || _loading) return;
	_loading = true;
	try {
		const data = await api.settings.complianceThresholds();
		if (data?.thresholds && data.thresholds.compliant > 0 && data.thresholds.warning > 0 && data.thresholds.compliant > data.thresholds.warning) {
			_thresholds = { compliant: data.thresholds.compliant, warning: data.thresholds.warning };
		}
	} catch (_) {
		// Use defaults
	} finally {
		_loaded = true;
		_loading = false;
	}
}

/**
 * Get current thresholds reactively.
 */
export function getThresholds() {
	return _thresholds;
}

/**
 * Score color based on current thresholds.
 * Returns CSS variable string.
 */
export function scoreColor(score) {
	const t = _thresholds;
	if (score === undefined || score === null) return 'var(--color-text-muted)';
	if (score >= t.compliant) return 'var(--color-success)';
	if (score >= t.warning) return 'var(--color-warning)';
	return 'var(--color-danger)';
}

/**
 * Score label based on current thresholds.
 */
export function scoreLabel(score) {
	const t = _thresholds;
	if (score === undefined || score === null) return 'Unscanned';
	if (score >= t.compliant) return 'Compliant';
	if (score >= t.warning) return 'Needs Improvement';
	return 'Critical';
}

/**
 * Threshold band range labels for display.
 */
export function bandLabel(band) {
	const t = _thresholds;
	switch (band) {
		case 'compliant': return `${t.compliant}–100%`;
		case 'warning':   return `${t.warning}–${t.compliant - 1}%`;
		case 'critical':   return `0–${t.warning - 1}%`;
		default:          return '—';
	}
}

export default { loadThresholds, getThresholds, scoreColor, scoreLabel, bandLabel };
