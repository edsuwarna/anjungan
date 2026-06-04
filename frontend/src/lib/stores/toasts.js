import { writable } from 'svelte/store';

/**
 * @typedef {Object} Toast
 * @property {string} id
 * @property {'success'|'error'|'info'|'warning'} type
 * @property {string} title
 * @property {string} message
 * @property {number} [duration=8000]
 */

/** @type {import('svelte/store').Writable<Toast[]>} */
export const toasts = writable([]);

let counter = 0;

/**
 * Add a toast notification.
 * @param {{ type: 'success'|'error'|'info'|'warning', title: string, message: string, duration?: number }} props
 * @returns {string} toast id
 */
export function addToast({ type, title, message, duration = 8000 }) {
	const id = `toast-${++counter}`;
	const toast = { id, type, title, message, duration };
	toasts.update(t => [...t, toast]);

	// Auto-dismiss
	setTimeout(() => {
		dismissToast(id);
	}, duration);

	return id;
}

/**
 * Remove a toast by id.
 * @param {string} id
 */
export function dismissToast(id) {
	toasts.update(t => t.filter(toast => toast.id !== id));
}
