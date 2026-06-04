<script>
	import Icon from '@iconify/svelte';
	import { toasts, dismissToast } from '$lib/stores/toasts.js';

	const typeColors = {
		success: { bg: '#059669', icon: 'solar:check-circle-bold' },
		error: { bg: '#dc2626', icon: 'solar:danger-triangle-bold' },
		warning: { bg: '#d97706', icon: 'solar:shield-warning-bold' },
		info: { bg: '#2563eb', icon: 'solar:info-circle-bold' },
	};
</script>

{#if $toasts.length > 0}
	<div class="fixed bottom-6 right-6 z-50 flex flex-col gap-2 max-w-sm">
		{#each $toasts as toast (toast.id)}
			<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
			<div
				class="flex items-start gap-3 px-4 py-3 rounded-lg shadow-lg text-white text-sm animate-slide-up cursor-pointer"
				style="background-color: {typeColors[toast.type]?.bg || '#374151'}; min-width: 280px;"
				onclick={() => dismissToast(toast.id)}
				role="alert"
			>
				<Icon icon={typeColors[toast.type]?.icon || 'solar:info-circle-bold'} class="h-5 w-5 shrink-0 mt-0.5" />
				<div class="flex-1 min-w-0">
					<p class="font-semibold text-sm">{toast.title}</p>
					<p class="text-xs opacity-90 mt-0.5">{toast.message}</p>
				</div>
				<button
					onclick={(e) => { e.stopPropagation(); dismissToast(toast.id); }}
					class="shrink-0 opacity-70 hover:opacity-100 transition-opacity"
				>
					<Icon icon="solar:close-circle-bold" class="h-4 w-4" />
				</button>
			</div>
		{/each}
	</div>
{/if}

<style>
	@keyframes slide-up {
		from { opacity: 0; transform: translateY(16px); }
		to { opacity: 1; transform: translateY(0); }
	}
	.animate-slide-up { animation: slide-up 0.3s ease-out; }
</style>
