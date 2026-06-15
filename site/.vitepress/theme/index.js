import DefaultTheme from 'vitepress/theme'
import './custom.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app, router }) {
    if (typeof window !== 'undefined') {
      const injectSidebarIcons = () => {
        if (document.getElementById('anj-sidebar-icons')) return
        const style = document.createElement('style')
        style.id = 'anj-sidebar-icons'
        style.textContent = `
/* ─── SECTION HEADER BASE STYLING (all level-0 items) ─── */
.VPSidebarItem.level-0 > .item > .text {
  padding-left: 24px !important;
  font-weight: 600 !important;
  font-size: 13px !important;
  letter-spacing: 0.02em !important;
  text-transform: uppercase !important;
  color: var(--vp-c-text-3) !important;
  background-repeat: no-repeat !important;
  background-position: left center !important;
  background-size: 16px 16px !important;
}
.VPSidebarItem.level-0.has-active > .item > .text {
  color: var(--vp-c-text-2) !important;
}

/* ─── ICONS per section ─── */
/* Getting Started (2nd .group child) */
.group:nth-child(2) .VPSidebarItem.level-0 > .item > .text {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%239c9c9d' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Ccircle cx='12' cy='12' r='10'/%3E%3Cpolygon points='16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76'/%3E%3C/svg%3E") !important;
}
.group:nth-child(2) .VPSidebarItem.level-0.has-active > .item > .text {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2310b981' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Ccircle cx='12' cy='12' r='10'/%3E%3Cpolygon points='16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76'/%3E%3C/svg%3E") !important;
}

/* Guides (3rd .group child) */
.group:nth-child(3) .VPSidebarItem.level-0 > .item > .text {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%239c9c9d' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z'/%3E%3Cpath d='M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z'/%3E%3C/svg%3E") !important;
}
.group:nth-child(3) .VPSidebarItem.level-0.has-active > .item > .text {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2310b981' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z'/%3E%3Cpath d='M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z'/%3E%3C/svg%3E") !important;
}

/* Reference (4th .group child) */
.group:nth-child(4) .VPSidebarItem.level-0 > .item > .text {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%239c9c9d' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z'/%3E%3Cpolyline points='14 2 14 8 20 8'/%3E%3Cline x1='16' y1='13' x2='8' y2='13'/%3E%3Cline x1='16' y1='17' x2='8' y2='17'/%3E%3C/svg%3E") !important;
}
.group:nth-child(4) .VPSidebarItem.level-0.has-active > .item > .text {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2310b981' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z'/%3E%3Cpolyline points='14 2 14 8 20 8'/%3E%3Cline x1='16' y1='13' x2='8' y2='13'/%3E%3Cline x1='16' y1='17' x2='8' y2='17'/%3E%3Csvg%3E") !important;
}
`
        document.head.appendChild(style)
      }
      if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', injectSidebarIcons)
      } else {
        injectSidebarIcons()
      }
      router.onAfterRouteChanged = () => {
        setTimeout(injectSidebarIcons, 50)
      }
    }
  }
}
