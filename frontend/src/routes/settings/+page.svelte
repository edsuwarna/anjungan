<script>
	import { onMount } from 'svelte';
import { api, setAuthToken } from '$lib/api.svelte.js';
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

	// 2FA TOTP
	let totpStep = $state('idle'); // 'idle' | 'setup' | 'enabled'
	let totpSaving = $state(false);
	let totpError = $state('');
	let totpSuccess = $state('');
	let totpSecret = $state('');
	let totpQRCode = $state('');
	let totpSetupToken = $state('');
	let disablePassword = $state('');

	onMount(() => {
		formName = $user?.name || '';
		formEmail = $user?.email || '';
		loading = false;

		totpStep = $user?.totp_enabled ? 'enabled' : 'idle';

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
			if (updated.access_token) {
				// Backend returned new token pair (email changed or always re-issued)
				localStorage.setItem('access_token', updated.access_token);
				localStorage.setItem('refresh_token', updated.refresh_token);
				localStorage.setItem('user', JSON.stringify(updated.user));
				setAuthToken(updated.access_token);
				user.set(updated.user);
			} else {
				// Fallback: old response shape (just user object)
				localStorage.setItem('user', JSON.stringify(updated));
				user.set(updated);
			}
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

	// ─── 2FA TOTP ──────────────────────────────────────────────────────────

	async function handleSetupTOTP() {
		totpError = '';
		totpSuccess = '';
		totpSaving = true;
		try {
			const data = await api.auth.setupTOTP();
			totpSecret = data.secret;
			totpQRCode = data.qr_code_base64;
			totpStep = 'setup';
		} catch (e) {
			totpError = e.message;
		} finally {
			totpSaving = false;
		}
	}

	async function handleVerifyTOTPSetup() {
		if (totpSetupToken.length < 6) return;
		totpError = '';
		totpSuccess = '';
		totpSaving = true;
		try {
			await api.auth.verifyTOTPSetup(totpSetupToken);
			totpSuccess = '2FA enabled successfully';
			totpStep = 'enabled';
			totpSetupToken = '';
			totpSecret = '';
			totpQRCode = '';
			// Update local user state
			const currentUser = $user;
			if (currentUser) {
				const updated = { ...currentUser, totp_enabled: true };
				localStorage.setItem('user', JSON.stringify(updated));
				user.set(updated);
			}
		} catch (e) {
			totpError = e.message;
		} finally {
			totpSaving = false;
		}
	}

	async function handleDisableTOTP() {
		if (!disablePassword) return;
		totpError = '';
		totpSuccess = '';
		totpSaving = true;
		try {
			await api.auth.disableTOTP({ password: disablePassword });
			totpSuccess = '2FA disabled successfully';
			totpStep = 'idle';
			disablePassword = '';
			// Update local user state
			const currentUser = $user;
			if (currentUser) {
				const updated = { ...currentUser, totp_enabled: false };
				localStorage.setItem('user', JSON.stringify(updated));
				user.set(updated);
			}
		} catch (e) {
			totpError = e.message;
		} finally {
			totpSaving = false;
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

		<!-- Two-Factor Authentication -->
		<div class="card">
			<div class="flex items-center gap-2 mb-4">
				<Icon icon="solar:shield-keyhole-bold" class="h-5 w-5" style="color: var(--color-primary);" />
				<h2 class="text-base font-semibold" style="color: var(--color-text);">Two-Factor Authentication</h2>
			</div>

			{#if totpError}
				<div class="mb-4 rounded-lg px-4 py-2 text-sm" style="background-color: var(--color-danger-subtle, #fee2e2); color: var(--color-danger);">{totpError}</div>
			{/if}
			{#if totpSuccess}
				<div class="mb-4 rounded-lg px-4 py-2 text-sm" style="background-color: var(--color-success-subtle, #d1fae5); color: var(--color-success);">{totpSuccess}</div>
			{/if}

			{#if totpStep === 'enabled'}
				<!-- 2FA is active -->
				<div class="flex items-center gap-2 mb-4">
					<Icon icon="solar:check-circle-bold" class="h-5 w-5" style="color: var(--color-success);" />
					<span class="text-sm font-medium" style="color: var(--color-success);">2FA is active</span>
				</div>

				<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Enter your password to disable 2FA</label>
				<input bind:value={disablePassword} type="password" class="input mb-4" placeholder="Current password" />

				<div class="flex justify-end">
					<button onclick={handleDisableTOTP} disabled={totpSaving || !disablePassword} class="btn-danger" style="background-color: var(--color-danger); color: white;">
						{totpSaving ? 'Disabling...' : 'Disable 2FA'}
					</button>
				</div>
			{:else if totpStep === 'setup'}
				<!-- QR Code shown, awaiting verification -->
				<p class="text-sm mb-3" style="color: var(--color-text-secondary);">
					Scan this QR code with your authenticator app (Google Authenticator, Authy, 1Password, etc.)
				</p>

				{#if totpQRCode}
					<div class="flex justify-center mb-4">
						<img src="data:image/png;base64,{totpQRCode}" alt="TOTP QR Code" class="rounded-lg" style="width: 192px; height: 192px;" />
					</div>
				{/if}

				<div class="mb-4 p-3 rounded-lg text-sm" style="background-color: var(--color-surface, #f1f5f9);">
					<p class="font-mono text-xs break-all" style="color: var(--color-text-muted);">Secret: {totpSecret}</p>
				</div>

				<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Verify with 6-digit code</label>
				<div class="flex gap-2">
					<input
						bind:value={totpSetupToken}
						type="text"
						inputmode="numeric"
						maxlength="6"
						class="input flex-1 text-center text-lg tracking-[0.3em]"
						placeholder="000000"
						onkeydown={(e) => { if (e.key === 'Enter') handleVerifyTOTPSetup(); }}
					/>
					<button
						onclick={handleVerifyTOTPSetup}
						disabled={totpSaving || totpSetupToken.length < 6}
						class="btn-primary"
					>
						{totpSaving ? 'Verifying...' : 'Verify'}
					</button>
				</div>

				<div class="mt-3">
					<button onclick={() => { totpStep = 'idle'; totpError = ''; totpSetupToken = ''; }} class="text-sm hover:opacity-80" style="color: var(--color-text-muted);">
						Cancel setup
					</button>
				</div>
			{:else}
				<!-- idle: 2FA not enabled -->
				<p class="text-sm mb-4" style="color: var(--color-text-secondary);">
					Add an extra layer of security to your account by enabling two-factor authentication.
				</p>
				<div class="flex justify-end">
					<button onclick={handleSetupTOTP} disabled={totpSaving} class="btn-primary">
						{totpSaving ? 'Preparing...' : 'Enable Two-Factor Auth'}
					</button>
				</div>
			{/if}
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
