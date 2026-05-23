/**
 * Screenshot capture script for tailrelay UI.
 * Uses Playwright with mocked API responses — no backend required.
 * Requires Vite dev server running: cd webui/frontend && npm run dev
 * Run: node take-screenshots.mjs
 */
import { chromium } from 'playwright';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dir = dirname(fileURLToPath(import.meta.url));
const OUT = __dir;
const BASE = 'http://localhost:5173';

// ── Mock data ────────────────────────────────────────────────────────────────

const MOCK_TCP_RELAYS = [
  { relay: { id: 'tcp-1', type: 'tcp', name: 'plex',      listen_port: 32400, target_host: '192.168.1.10', target_port: 32400, enabled: true,  autostart: true  }, running: true  },
  { relay: { id: 'tcp-2', type: 'tcp', name: 'minecraft', listen_port: 25565, target_host: '192.168.1.20', target_port: 25565, enabled: false, autostart: false }, running: false },
  { relay: { id: 'tcp-3', type: 'tcp', name: 'jellyfin',  listen_port: 8096,  target_host: '192.168.1.10', target_port: 8096,  enabled: true,  autostart: true  }, running: true  },
];

const MOCK_HTTPS_RELAYS = [
  { id: 'https-1', type: 'https', hostname: 'grafana.homelab.ts.net',   listen_port: 443,  target_host: '192.168.1.10', target_port: 3000, enabled: true,  autostart: true,  running: true  },
  { id: 'https-2', type: 'https', hostname: 'portainer.homelab.ts.net', listen_port: 8443, target_host: '192.168.1.10', target_port: 9000, enabled: true,  autostart: true,  running: true  },
  { id: 'https-3', type: 'https', hostname: 'nginx.homelab.ts.net',     listen_port: 7443, target_host: '192.168.1.30', target_port: 80,   enabled: false, autostart: false, running: false },
];

const MOCK_TS_STATUS = {
  BackendState: 'Running',
  Self: { HostName: 'tailrelay', TailscaleIPs: ['100.97.110.112'], DNSName: 'tailrelay.homelab.ts.net.', OS: 'linux' },
  MagicDNSName: 'tailrelay.homelab.ts.net.',
  MagicDNSSuffix: 'homelab.ts.net',
  PeerCount: 7,
  ActivePeers: 5,
  CurrentTailnet: { Name: 'homelab.ts.net', MagicDNSEnabled: true, MagicDNSSuffix: 'homelab.ts.net' },
  Version: '1.76.6',
  Health: [],
};

const MOCK_TS_PEERS = [
  { DNSName: 'macbook-pro.homelab.ts.net.', Hostname: 'macbook-pro', TailscaleIPs: ['100.97.110.50'], IPv4: '100.97.110.50', OS: 'macos',   Active: true,  Online: true,  LastSeen: new Date().toISOString() },
  { DNSName: 'iphone.homelab.ts.net.',      Hostname: 'iphone',      TailscaleIPs: ['100.97.110.75'], IPv4: '100.97.110.75', OS: 'ios',     Active: false, Online: true,  LastSeen: new Date(Date.now() - 120000).toISOString() },
  { DNSName: 'home-server.homelab.ts.net.', Hostname: 'home-server', TailscaleIPs: ['100.97.110.30'], IPv4: '100.97.110.30', OS: 'linux',   Active: true,  Online: true,  LastSeen: new Date().toISOString() },
  { DNSName: 'proxmox-ve.homelab.ts.net.',  Hostname: 'proxmox-ve',  TailscaleIPs: ['100.97.110.20'], IPv4: '100.97.110.20', OS: 'linux',   Active: true,  Online: true,  LastSeen: new Date().toISOString() },
  { DNSName: 'nas-storage.homelab.ts.net.', Hostname: 'nas-storage', TailscaleIPs: ['100.97.110.25'], IPv4: '100.97.110.25', OS: 'freebsd', Active: true,  Online: true,  LastSeen: new Date().toISOString() },
  { DNSName: 'ipad.homelab.ts.net.',        Hostname: 'ipad',        TailscaleIPs: ['100.97.110.76'], IPv4: '100.97.110.76', OS: 'ios',     Active: false, Online: false, LastSeen: new Date(Date.now() - 3600000).toISOString() },
  { DNSName: 'work-laptop.homelab.ts.net.', Hostname: 'work-laptop', TailscaleIPs: ['100.97.110.90'], IPv4: '100.97.110.90', OS: 'windows', Active: false, Online: false, LastSeen: new Date(Date.now() - 86400000).toISOString() },
];

const MOCK_BACKUPS = [
  { filename: 'tailrelay-full-2026-03-13T10-00-00.tar.gz', size: 48320, date: '2026-03-13T10:00:00Z', type: 'full' },
  { filename: 'tailrelay-full-2026-03-12T10-00-00.tar.gz', size: 47918, date: '2026-03-12T10:00:00Z', type: 'full' },
  { filename: 'tailrelay-full-2026-03-11T10-00-00.tar.gz', size: 46204, date: '2026-03-11T10:00:00Z', type: 'full' },
];

const MOCK_LOGS = [
  { level: 'INFO',  message: 'serve relay started: plex (tcp :32400 → 192.168.1.10:32400)',    timestamp: new Date().toISOString() },
  { level: 'INFO',  message: 'serve relay started: jellyfin (tcp :8096 → 192.168.1.10:8096)',  timestamp: new Date().toISOString() },
  { level: 'WARN',  message: 'Tailscale: peer work-laptop went offline',                        timestamp: new Date(Date.now() - 60000).toISOString() },
  { level: 'INFO',  message: 'serve relay started: grafana (https :443 → 192.168.1.10:3000)',   timestamp: new Date(Date.now() - 120000).toISOString() },
  { level: 'ERROR', message: 'Failed to reload tailscale serve config: timeout',                timestamp: new Date(Date.now() - 180000).toISOString() },
  { level: 'DEBUG', message: 'Polling tailscale status',                                        timestamp: new Date(Date.now() - 240000).toISOString() },
];

const MOCK_TARGETS = [
  { label: 'Plex',       value: '192.168.1.10:32400' },
  { label: 'Jellyfin',   value: '192.168.1.10:8096'  },
  { label: 'Grafana',    value: '192.168.1.10:3000'  },
  { label: 'Portainer',  value: '192.168.1.10:9000'  },
];

// ── Helpers ──────────────────────────────────────────────────────────────────

async function setupMocks(page, { authenticated = true } = {}) {
  page.on('console', msg => console.log('PAGE LOG:', msg.text()));
  page.on('pageerror', err => console.error('PAGE ERROR:', err.message));

  await page.route('**/api/auth/status',      r => r.fulfill({ contentType: 'application/json', body: JSON.stringify({ authenticated, needsSetup: false }) }));
  await page.route('**/api/auth/login',       r => r.fulfill({ contentType: 'application/json', body: JSON.stringify({ ok: true }) }));
  await page.route('**/api/serve/tcp/list',   r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_TCP_RELAYS) }));
  await page.route('**/api/serve/https/list', r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_HTTPS_RELAYS) }));
  await page.route('**/api/tailscale/status', r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_TS_STATUS) }));
  await page.route('**/api/tailscale/peers',  r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_TS_PEERS) }));
  await page.route('**/api/backup/list',      r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_BACKUPS) }));
  await page.route('**/api/logs',             r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_LOGS) }));
  await page.route('**/api/logs/stream',      r => r.abort());
  await page.route('**/api/targets',          r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_TARGETS) }));
  await page.route('**/api/info',             r => r.fulfill({ contentType: 'application/json', body: JSON.stringify({ version: 'v0.7.0' }) }));
}

async function setTheme(page, mode) {
  await page.evaluate((m) => {
    localStorage.setItem('theme', m);
    document.documentElement.classList.toggle('dark', m === 'dark');
  }, mode);
  await page.waitForTimeout(300);
}

async function snap(page, name) {
  const path = resolve(OUT, `${name}.png`);
  await page.screenshot({ path, fullPage: false });
  console.log(`  saved ${name}.png`);
}

// ── Main ─────────────────────────────────────────────────────────────────────

const browser = await chromium.launch({
  executablePath: `${process.env.HOME}/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome`,
  headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox'],
});

const context = await browser.newContext();

// ════════════════════════════════════════════════════════════════════════════
// 1. LOGIN PAGE
// ════════════════════════════════════════════════════════════════════════════
console.log('\n[1/4] Login page');
{
  const page = await context.newPage();
  await setupMocks(page, { authenticated: false });

  // Desktop light
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto(BASE, { waitUntil: 'networkidle' });
  await setTheme(page, 'light');
  await page.waitForTimeout(400);
  await snap(page, 'login-light-desktop');

  // Desktop dark
  await setTheme(page, 'dark');
  await snap(page, 'login-dark-desktop');

  // Mobile light
  await page.setViewportSize({ width: 390, height: 844 });
  await setTheme(page, 'light');
  await snap(page, 'login-light-mobile');

  // Mobile dark
  await setTheme(page, 'dark');
  await snap(page, 'login-dark-mobile');

  await page.close();
}

// ════════════════════════════════════════════════════════════════════════════
// 2. DASHBOARD
// ════════════════════════════════════════════════════════════════════════════
console.log('\n[2/4] Dashboard');
{
  // Desktop
  const page = await context.newPage();
  await setupMocks(page);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(BASE, { waitUntil: 'networkidle' });
  await page.waitForTimeout(800);

  await setTheme(page, 'light');
  await snap(page, 'dashboard-light-desktop');

  await setTheme(page, 'dark');
  await snap(page, 'dashboard-dark-desktop');

  // Log console expanded
  await setTheme(page, 'dark');
  const logSection = page.locator('button').filter({ hasText: /logs/i }).last();
  try {
    await logSection.click({ timeout: 2000 });
    await page.waitForTimeout(400);
  } catch {
    const buttons = await page.locator('button').all();
    for (const btn of buttons) {
      const text = await btn.textContent();
      if (text && /log|console|terminal/i.test(text)) {
        await btn.click().catch(() => {});
        await page.waitForTimeout(300);
        break;
      }
    }
  }
  await snap(page, 'dashboard-logs-dark-desktop');

  await page.close();

  // Mobile
  const page2 = await context.newPage();
  await setupMocks(page2);
  await page2.setViewportSize({ width: 390, height: 844 });
  await page2.goto(BASE, { waitUntil: 'networkidle' });
  await page2.waitForTimeout(800);

  await setTheme(page2, 'light');
  await snap(page2, 'dashboard-light-mobile');

  await setTheme(page2, 'dark');
  await snap(page2, 'dashboard-dark-mobile');

  // Mobile menu open (light)
  await setTheme(page2, 'light');
  await page2.evaluate(() => {
    const hamburger = Array.from(document.querySelectorAll('nav button')).find(b => b.className.includes('sm:hidden'));
    if (hamburger) hamburger.click();
  });
  await page2.waitForTimeout(300);
  await snap(page2, 'dashboard-mobile-menu-light');

  // Mobile menu open (dark)
  await setTheme(page2, 'dark');
  await snap(page2, 'dashboard-mobile-menu-dark');

  await page2.close();
}

// ════════════════════════════════════════════════════════════════════════════
// 3. TAILSCALE TAB
// ════════════════════════════════════════════════════════════════════════════
console.log('\n[3/4] Tailscale tab');
{
  const page = await context.newPage();
  await setupMocks(page);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(BASE, { waitUntil: 'networkidle' });
  await page.waitForTimeout(800);

  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll('nav button')).find(b => b.textContent.trim().startsWith('Tailscale'));
    if (btn) btn.click();
  });
  await page.waitForTimeout(600);

  await setTheme(page, 'light');
  await snap(page, 'tailscale-light-desktop');

  await setTheme(page, 'dark');
  await snap(page, 'tailscale-dark-desktop');

  // Mobile
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(300);

  await setTheme(page, 'light');
  await snap(page, 'tailscale-light-mobile');

  await setTheme(page, 'dark');
  await snap(page, 'tailscale-dark-mobile');

  await page.close();
}

// ════════════════════════════════════════════════════════════════════════════
// 4. BACKUPS TAB
// ════════════════════════════════════════════════════════════════════════════
console.log('\n[4/4] Backups tab');
{
  const page = await context.newPage();
  await setupMocks(page);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(BASE, { waitUntil: 'networkidle' });
  await page.waitForTimeout(800);

  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll('nav button')).find(b => b.textContent.trim().startsWith('Backups'));
    if (btn) btn.click();
  });
  await page.waitForTimeout(600);

  await setTheme(page, 'light');
  await snap(page, 'backups-light-desktop');

  await setTheme(page, 'dark');
  await snap(page, 'backups-dark-desktop');

  // Mobile
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(300);
  await snap(page, 'backups-dark-mobile');

  await page.close();
}

await browser.close();
console.log('\nDone! All screenshots saved to docs/screenshots/');
