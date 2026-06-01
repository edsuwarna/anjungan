export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	theme: {
		extend: {
			colors: {
				// Emerald palette — primary
				primary: {
					50: '#ecfdf5',
					100: '#d1fae5',
					200: '#a7f3d0',
					300: '#6ee7b7',
					400: '#34d399',
					500: '#10b981',
					600: '#059669',
					700: '#047857',
					800: '#065f46',
					900: '#064e3b',
					950: '#022c22'
				},
				// Surface — off-white for light, dark gray for dark
				surface: {
					light: '#f4f5f7',
					DEFAULT: '#f4f5f7',
					dark: '#1a1d23'
				},
				sidebar: {
					light: '#ffffff',
					DEFAULT: '#ffffff',
					dark: '#111318'
				},
				border: {
					light: '#e5e7eb',
					DEFAULT: '#e5e7eb',
					dark: '#2a2d35'
				}
			},
			fontFamily: {
				sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
				mono: ['JetBrains Mono', 'Fira Code', 'monospace']
			}
		}
	},
	plugins: [require('@tailwindcss/forms'), require('@tailwindcss/typography'), require('@tailwindcss/container-queries')]
};
