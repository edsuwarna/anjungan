<script>
	import Icon from '@iconify/svelte';
	let { title = '', value = 0, icon = '', subtitle = '', style = '' } = $props();
</script>

<div class="stat-card" {style}>
	<div class="flex items-center justify-between">
		<div>
			<p class="text-sm font-medium" style="color: var(--color-text-secondary);">{title}</p>
			<p class="mt-1 text-2xl font-bold" style="color: var(--color-text);">{value}</p>
			{#if subtitle}
				<p class="mt-0.5 text-xs" style="color: var(--color-text-muted);">{subtitle}</p>
			{/if}
		</div>
		{#if icon}
			<div
				class="flex h-10 w-10 items-center justify-center rounded-lg"
				style="background-color: var(--color-primary-subtle);"
			>
				<Icon icon={icon} class="h-5 w-5" style="color: var(--color-primary);" />
			</div>
		{/if}
	</div>
</div>
