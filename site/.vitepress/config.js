import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Anjungan',
  description: 'A modular internal developer platform (IDP) for managing servers, containers, container registries, and infrastructure compliance through a unified dashboard.',
  lang: 'en-US',
  ignoreDeadLinks: true,

  appearance: 'dark',
  lastUpdated: true,

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: 'data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 32 32%22><circle cx=%2216%22 cy=%2216%22 r=%2214%22 fill=%22%2357c1ff%22/></svg>' }],
    ['script', {}, `
document.addEventListener('DOMContentLoaded', function() {
  var track = document.getElementById('carouselTrack');
  var prev = document.getElementById('carouselPrev');
  var next = document.getElementById('carouselNext');
  var dots = document.querySelectorAll('.anj-dot');
  if (!track || !prev || !next) return;
  var current = 0;
  var total = track.children.length;
  function goTo(i) {
    if (i < 0) i = total - 1;
    if (i >= total) i = 0;
    current = i;
    track.style.transform = 'translateX(-' + (current * 100) + '%)';
    dots.forEach(function(d) { d.classList.remove('active'); });
    if (dots[current]) dots[current].classList.add('active');
  }
  prev.addEventListener('click', function() { goTo(current - 1); });
  next.addEventListener('click', function() { goTo(current + 1); });
  dots.forEach(function(dot) {
    dot.addEventListener('click', function() {
      goTo(parseInt(this.getAttribute('data-slide')));
    });
  });
  setInterval(function() { goTo(current + 1); }, 4000);
});
`],
  ],

  themeConfig: {
    logo: { src: '/anjungan-icon.svg', width: 28, height: 28 },

    nav: [
      { text: 'Home', link: '/' },
      { text: 'Docs', link: '/guide/' },
      { text: 'API', link: '/guide/api' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Getting Started',
          items: [
            { text: 'Overview', link: '/guide/' },
            { text: 'Setup', link: '/guide/setup' },
            { text: 'Architecture', link: '/guide/architecture' },
          ],
        },
        {
          text: 'Guides',
          items: [
            { text: 'Deployment', link: '/guide/deployment' },
            { text: 'Container Registry', link: '/guide/registry' },
            { text: 'Compliance', link: '/guide/compliance' },
            { text: 'Self-Server Registration', link: '/guide/self-server' },
          ],
        },
        {
          text: 'Reference',
          items: [
            { text: 'API Reference', link: '/guide/api' },
            { text: 'Docker', link: '/guide/docker' },
          ],
        },
      ],
    },

    socialLinks: [],

    footer: {
      copyright: 'Copyright © 2024-2026 Endang Suwarna',
    },

    search: {
      provider: 'local',
    },
  },
})
