<script>
	import { user } from '$lib/stores/auth.js';
</script>

<header
	class="flex h-16 items-center justify-between border-b px-6"
	style="background-color: var(--color-sidebar); border-color: var(--color-border);"
>
	<div class="flex items-center gap-3">
		<h2 class="text-lg font-semibold">Overview</h2>
	</div>

	<div class="flex items-center gap-4">
		<div class="flex h-8 w-8 items-center justify-center rounded-full" style="background-color: var(--color-primary);">
			<span class="text-xs font-bold text-white">
				{$user?.name?.charAt(0) || 'A'}
			</span>
		</div>
	</div>
</header>
