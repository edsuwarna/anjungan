<script>
	let { title = '', value = 0, icon = '', color = '#10b981' } = $props();
</script>

<div
	class="rounded-xl border p-5 transition-all hover:shadow-md animate-fade-in"
	style="background-color: var(--color-sidebar); border-color: var(--color-border);"
>
	<div class="flex items-center justify-between">
		<div>
			<p class="text-sm font-medium" style="color: var(--color-text-secondary);">{title}</p>
			<p class="mt-1 text-2xl font-bold">{value}</p>
		</div>
		<div class="flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: {color}15;">
			<span class="icon-[{icon}] h-5 w-5" style="color: {color};"></span>
		</div>
	</div>
</div>
