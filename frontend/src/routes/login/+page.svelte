<script>
	import { api } from '$lib/api.js';
	import { user } from '$lib/stores/auth.js';
	import { goto } from '$app/navigation';

	let email = '';
	let password = '';
	let loading = false;
	let error = '';

	async function handleLogin() {
		loading = true;
		error = '';
		try {
			const data = await api.auth.login(email, password);
			localStorage.setItem('access_token', data.access_token);
			localStorage.setItem('refresh_token', data.refresh_token);
			user.set(data.user);
			goto('/');
		} catch (e) {
			error = e.message || 'Login failed';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center" style="background-color: var(--color-surface);">
	<div class="w-full max-w-sm rounded-2xl border p-8 shadow-lg animate-fade-in"
		style="background-color: var(--color-sidebar); border-color: var(--color-border);"
	>
		<div class="mb-8 text-center">
			<div class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl" style="background-color: var(--color-primary);">
				<span class="text-xl font-bold text-white">A</span>
			</div>
			<h1 class="text-xl font-bold">Welcome to Anjungan</h1>
			<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">Sign in to your platform</p>
		</div>

		<form onsubmit={handleLogin}>
			<div class="space-y-4">
				<div>
					<label for="email" class="mb-1 block text-sm font-medium">Email</label>
					<input
						id="email"
						type="email"
						bind:value={email}
						required
						class="w-full rounded-lg border px-3 py-2.5 text-sm outline-none transition-colors focus:ring-2"
						style="border-color: var(--color-border); background-color: var(--color-surface); color: var(--color-text);"
						placeholder="admin@anjungan.id"
					/>
				</div>

				<div>
					<label for="password" class="mb-1 block text-sm font-medium">Password</label>
					<input
						id="password"
						type="password"
						bind:value={password}
						required
						class="w-full rounded-lg border px-3 py-2.5 text-sm outline-none transition-colors focus:ring-2"
						style="border-color: var(--color-border); background-color: var(--color-surface); color: var(--color-text);"
						placeholder="••••••••"
					/>
				</div>

				{#if error}
					<div class="rounded-lg px-4 py-3 text-sm" style="background-color: rgba(239, 68, 68, 0.1); color: var(--color-danger);">
						{error}
					</div>
				{/if}

				<button
					type="submit"
					disabled={loading}
					class="w-full rounded-lg px-4 py-2.5 text-sm font-semibold text-white transition-all disabled:opacity-50"
					style="background-color: var(--color-primary);"
				>
					{loading ? 'Signing in...' : 'Sign in'}
				</button>
			</div>
		</form>
	</div>
</div>
