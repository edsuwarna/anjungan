import { writable } from 'svelte/store';

export const user = writable(null);
export const theme = writable('light');
export const sidebarCollapsed = writable(false);
