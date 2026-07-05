import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';
import type * as OpenApiPlugin from 'docusaurus-plugin-openapi-docs';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const config: Config = {
  title: 'tailrelay',
  tagline: 'Expose local services to your Tailscale network',
  favicon: 'img/favicon.ico',

  future: {
    v4: true, // Improve compatibility with the upcoming Docusaurus v4
  },

  // GitHub Pages project site: https://sudocarlos.github.io/tailrelay/
  url: 'https://sudocarlos.github.io',
  baseUrl: '/tailrelay/',

  organizationName: 'sudocarlos',
  projectName: 'tailrelay',

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          docItemComponent: '@theme/ApiItem',
          editUrl: 'https://github.com/sudocarlos/tailrelay/tree/main/website/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    [
      'docusaurus-plugin-openapi-docs',
      {
        id: 'api',
        docsPluginId: 'classic',
        config: {
          tailrelay: {
            specPath: '../docs/openapi.yaml',
            outputDir: 'docs/api',
            sidebarOptions: {
              groupPathsBy: 'tag',
            },
          } satisfies OpenApiPlugin.Options,
        },
      },
    ],
  ],

  themes: ['docusaurus-theme-openapi-docs'],

  themeConfig: {
      image: 'img/social-card.png',
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'tailrelay',
      logo: {
        alt: 'tailrelay logo',
        src: 'img/logo.png',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          to: '/docs/api/',
          label: 'API Reference',
          position: 'left',
        },
        {
          href: 'https://github.com/sudocarlos/tailrelay',
          label: 'GitHub',
          position: 'right',
        },
        {
          href: 'https://hub.docker.com/r/sudocarlos/tailrelay',
          label: 'Docker Hub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {label: 'Getting Started', to: '/docs/getting-started'},
            {label: 'API Reference', to: '/docs/api/'},
            {label: 'Troubleshooting', to: '/docs/troubleshooting'},
          ],
        },
        {
          title: 'Project',
          items: [
            {label: 'GitHub', href: 'https://github.com/sudocarlos/tailrelay'},
            {label: 'Issues', href: 'https://github.com/sudocarlos/tailrelay/issues'},
            {
              label: 'Changelog',
              href: 'https://github.com/sudocarlos/tailrelay/blob/main/CHANGELOG.md',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {label: 'Docker Hub', href: 'https://hub.docker.com/r/sudocarlos/tailrelay'},
            {label: 'Tailscale', href: 'https://tailscale.com'},
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} tailrelay. Licensed under BSD-3-Clause.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
