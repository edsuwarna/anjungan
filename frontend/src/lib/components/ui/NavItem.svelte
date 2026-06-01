<script>
	let { href = '', title = '' } = $props();
</script>

<a
	{href}
	class="flex items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium transition-all hover:opacity-80"
	style="color: var(--color-text-secondary); background-color: var(--color-surface);"
>
	<slot />
	{title}
</a>
