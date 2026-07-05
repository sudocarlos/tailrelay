import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const sidebars: SidebarsConfig = {
  docsSidebar: [
    'intro',
    'getting-started',
    'authentication',
    'development',
    'troubleshooting',
  ],
};

export default sidebars;
