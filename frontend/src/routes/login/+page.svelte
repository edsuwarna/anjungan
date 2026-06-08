<script>
	import Icon from '@iconify/svelte';
	import { api, setAuthToken } from '$lib/api.svelte.js';
	import { user } from '$lib/stores/auth.js';
	import { goto } from '$app/navigation';

	let email = $state('');
	let password = $state('');
	let name = $state('');
	let loading = $state(false);
	let error = $state('');
	let showPassword = $state(false);
	let mode = $state('login'); // 'login' | 'register'

	async function handleSubmit(e) {
		e.preventDefault();
		loading = true;
		error = '';

		try {
			if (mode === 'login') {
				const data = await api.auth.login(email, password);
				localStorage.setItem('access_token', data.access_token);
				localStorage.setItem('refresh_token', data.refresh_token);
				setAuthToken(data.access_token);
				user.set(data.user);
				localStorage.setItem('user', JSON.stringify(data.user));
				goto('/');
			} else {
				await api.auth.register(email, name, password);
				// Auto-navigate to login after successful registration
				mode = 'login';
				error = '';
				// Clear register fields but keep email
				name = '';
				password = '';
			}
		} catch (e) {
			error = e.message || (mode === 'login' ? 'Login failed' : 'Registration failed');
		} finally {
			loading = false;
		}
	}

	function switchMode() {
		mode = mode === 'login' ? 'register' : 'login';
		error = '';
		password = '';
		name = '';
	}
</script>

<div class="flex w-full min-h-screen items-center justify-center bg-gradient-to-br from-teal-50 to-slate-100 dark:from-slate-950 dark:to-slate-900">
	<div class="w-full max-w-sm">
		<!-- Card -->
		<div class="rounded-2xl border p-8 shadow-lg animate-fade-in"
			style="background-color: var(--color-card); border-color: var(--color-border-light);"
		>
			<!-- Logo -->
			<div class="mb-8 text-center">
				<div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-xl shadow-sm"
					style="background: linear-gradient(135deg, var(--color-primary), var(--color-primary-hover));"
				>
					<span class="text-xl font-bold text-white">A</span>
				</div>
				<h1 class="text-xl font-bold" style="color: var(--color-text);">Anjungan</h1>
				<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
					{mode === 'login' ? 'Sign in to your platform' : 'Create your account'}
				</p>
			</div>

			<!-- Form -->
			<form onsubmit={handleSubmit} class="space-y-4">
				{#if mode === 'register'}
					<div>
						<label for="name" class="mb-1.5 block text-sm font-medium" style="color: var(--color-text);">Name</label>
						<input
							id="name"
							type="text"
							bind:value={name}
							required
							class="input"
							placeholder="Your name"
						/>
					</div>
				{/if}

				<div>
					<label for="email" class="mb-1.5 block text-sm font-medium" style="color: var(--color-text);">Email</label>
					<input
						id="email"
						type="email"
						bind:value={email}
						required
						class="input"
						placeholder="your-name@example.com"
					/>
				</div>

				<div>
					<label for="password" class="mb-1.5 block text-sm font-medium" style="color: var(--color-text);">Password</label>
					<div class="relative">
						<input
							id="password"
							type={showPassword ? 'text' : 'password'}
							bind:value={password}
							required
							class="input w-full pr-10"
							placeholder="••••••••"
							minlength={mode === 'register' ? 8 : undefined}
						/>
						<button
							type="button"
							onclick={() => showPassword = !showPassword}
							class="absolute right-3 top-1/2 -translate-y-1/2 flex items-center justify-center p-1 rounded-md hover:opacity-80"
							style="color: var(--color-text-muted);"
							aria-label={showPassword ? 'Hide password' : 'Show password'}
						>
							{#if showPassword}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/>
									<path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/>
									<line x1="1" y1="1" x2="23" y2="23"/>
									<path d="M14.12 14.12a3 3 0 1 1-4.24-4.24"/>
								</svg>
							{:else}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
									<circle cx="12" cy="12" r="3"/>
								</svg>
							{/if}
						</button>
					</div>
				</div>

				{#if error}
					<div class="rounded-lg px-4 py-3 text-sm" style="background-color: rgba(239, 68, 68, 0.1); color: var(--color-danger);">
						{error}
					</div>
				{/if}

				<button
					type="submit"
					disabled={loading}
					class="btn-primary w-full py-2.5"
				>
					{#if loading}
						<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" />
						{mode === 'login' ? 'Signing in...' : 'Creating account...'}
					{:else}
						{mode === 'login' ? 'Sign in' : 'Create Account'}
					{/if}
				</button>
			</form>

			<!-- Mode switch -->
			<div class="mt-6 text-center">
				<button
					type="button"
					onclick={switchMode}
					class="text-sm font-medium transition-colors hover:opacity-80"
					style="color: var(--color-primary);"
				>
					{mode === 'login'
						? "Don't have an account? Register"
						: 'Already have an account? Sign in'}
				</button>
			</div>
		</div>

		<!-- Footer -->
		<p class="mt-6 text-center text-xs" style="color: var(--color-text-muted);">
			Anjungan &mdash; Platform Engineering Dashboard
		</p>
	</div>
</div>
