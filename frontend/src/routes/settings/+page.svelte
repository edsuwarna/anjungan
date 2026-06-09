<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import { user } from '$lib/stores/auth.js';
	import Icon from '@iconify/svelte';

	let loading = $state(true);

	// Profile form
	let formName = $state('');
	let formEmail = $state('');
	let profileSaving = $state(false);
	let profileError = $state('');
	let profileSuccess = $state('');

	// Password form
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let passwordSaving = $state(false);
	let passwordError = $state('');
	let passwordSuccess = $state('');

	// Registration setting (admin)
	let registrationEnabled = $state(false);
	let regSaving = $state(false);
	let regError = $state('');

	onMount(() => {
		formName = $user?.name || '';
		formEmail = $user?.email || '';
		loading = false;

		if ($user?.role === 'admin') {
			loadRegistration();
		}
	});

	async function loadRegistration() {
		try {
			const data = await api.settings.registration();
			registrationEnabled = data.enabled;
		} catch (_) {}
	}

	async function handleProfileUpdate() {
		profileError = '';
		profileSuccess = '';
		profileSaving = true;
		try {
			const payload = {};
			if (formName !== ($user?.name || '')) payload.name = formName;
			if (formEmail !== ($user?.email || '')) payload.email = formEmail;
			if (Object.keys(payload).length === 0) {
				profileError = 'No changes to save';
				profileSaving = false;
				return;
			}
			const updated = await api.auth.updateProfile(payload);
			localStorage.setItem('user', JSON.stringify(updated));
			user.set(updated);
			profileSuccess = 'Profile updated';
		} catch (e) {
			profileError = e.message;
		} finally {
			profileSaving = false;
		}
	}

	async function handlePasswordChange() {
		passwordError = '';
		passwordSuccess = '';
		if (newPassword !== confirmPassword) {
			passwordError = 'Passwords do not match';
			return;
		}
		if (newPassword.length < 6) {
			passwordError = 'Password must be at least 6 characters';
			return;
		}
		passwordSaving = true;
		try {
			await api.auth.changePassword({
				current_password: currentPassword,
				new_password: newPassword,
			});
			passwordSuccess = 'Password changed successfully';
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
		} catch (e) {
			passwordError = e.message;
		} finally {
			passwordSaving = false;
		}
	}

	async function handleRegistrationToggle() {
		regError = '';
		regSaving = true;
		try {
			await api.settings.updateRegistration({ enabled: !registrationEnabled });
			registrationEnabled = !registrationEnabled;
		} catch (e) {
			regError = e.message;
		} finally {
			regSaving = false;
		}
	}
</script>

<div class="page-container">
	<h1 class="page-title">Settings</h1>
	<p class="page-subtitle">Manage your account and preferences</p>

	<div class="space-y-6 mt-6">
		<!-- Profile Section -->
		<div class="card">
			<div class="flex items-center gap-2 mb-4">
				<Icon icon="solar:user-id-bold" class="h-5 w-5" style="color: var(--color-primary);" />
				<h2 class="text-base font-semibold" style="color: var(--color-text);">Profile</h2>
			</div>

			{#if profileSuccess}
				<div class="mb-4 rounded-lg px-4 py-2 text-sm" style="background-color: var(--color-success-subtle, #d1fae5); color: var(--color-success);">{profileSuccess}</div>
			{/if}
			{#if profileError}
				<div class="mb-4 rounded-lg px-4 py-2 text-sm" style="background-color: var(--color-danger-subtle, #fee2e2); color: var(--color-danger);">{profileError}</div>
			{/if}

			<div class="space-y-4">
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Name</label>
					<input bind:value={formName} class="input" placeholder="Your name" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Email</label>
					<input bind:value={formEmail} type="email" class="input" placeholder="email@example.com" />
				</div>
				<div class="flex justify-end">
					<button onclick={handleProfileUpdate} disabled={profileSaving} class="btn-primary">
						{profileSaving ? 'Saving...' : 'Save Profile'}
					</button>
				</div>
			</div>
		</div>

		<!-- Change Password Section -->
		<div class="card">
			<div class="flex items-center gap-2 mb-4">
				<Icon icon="solar:lock-password-bold" class="h-5 w-5" style="color: var(--color-primary);" />
				<h2 class="text-base font-semibold" style="color: var(--color-text);">Change Password</h2>
			</div>

			{#if passwordSuccess}
				<div class="mb-4 rounded-lg px-4 py-2 text-sm" style="background-color: var(--color-success-subtle, #d1fae5); color: var(--color-success);">{passwordSuccess}</div>
			{/if}
			{#if passwordError}
				<div class="mb-4 rounded-lg px-4 py-2 text-sm" style="background-color: var(--color-danger-subtle, #fee2e2); color: var(--color-danger);">{passwordError}</div>
			{/if}

			<div class="space-y-4">
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Current Password</label>
					<input bind:value={currentPassword} type="password" class="input" placeholder="Enter current password" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">New Password</label>
					<input bind:value={newPassword} type="password" class="input" placeholder="Min 6 characters" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Confirm New Password</label>
					<input bind:value={confirmPassword} type="password" class="input" placeholder="Repeat new password" />
				</div>
				<div class="flex justify-end">
					<button onclick={handlePasswordChange} disabled={passwordSaving || !currentPassword || !newPassword || !confirmPassword} class="btn-primary">
						{passwordSaving ? 'Changing...' : 'Change Password'}
					</button>
				</div>
			</div>
		</div>

		<!-- Admin: Registration Toggle -->
		{#if $user?.role === 'admin'}
			<div class="card">
				<div class="flex items-center justify-between">
					<div class="flex items-center gap-2">
						<Icon icon="solar:user-plus-bold" class="h-5 w-5" style="color: var(--color-primary);" />
						<div>
							<h2 class="text-base font-semibold" style="color: var(--color-text);">Public Registration</h2>
							<p class="text-sm mt-0.5" style="color: var(--color-text-secondary);">Allow new users to register via the login page</p>
						</div>
					</div>
					<button
						onclick={handleRegistrationToggle}
						disabled={regSaving}
						class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
						style="background-color: {registrationEnabled ? 'var(--color-success)' : 'var(--color-border)'};"
						role="switch"
						aria-checked={registrationEnabled}
					>
						<span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform"
							class:translate-x-6={registrationEnabled}
							class:translate-x-1={!registrationEnabled}
						/>
					</button>
				</div>
				{#if regError}
					<p class="mt-2 text-sm" style="color: var(--color-danger);">{regError}</p>
				{/if}
			</div>
		{/if}
	</div>
</div>
