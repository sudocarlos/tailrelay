/**
 * Screenshot capture script for tailrelay UI.
 * Uses Playwright with mocked API responses — no backend required.
 * Run: node take-screenshots.mjs
 */
import { chromium } from 'playwright';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dir = dirname(fileURLToPath(import.meta.url));
const OUT = __dir;
const BASE = 'http://localhost:5173';

// ── Mock data ────────────────────────────────────────────────────────────────

const MOCK_RELAYS = [
  { Relay: { id: 'relay-1', listen_port: 32400, target_host: '192.168.1.10', target_port: 32400, name: 'plex', auto: true },  Running: true },
  { Relay: { id: 'relay-2', listen_port: 25565, target_host: '192.168.1.20', target_port: 25565, name: 'minecraft', auto: false }, Running: false },
  { Relay: { id: 'relay-3', listen_port: 8096,  target_host: '192.168.1.10', target_port: 8096,  name: 'jellyfin', auto: true },  Running: true },
];

const MOCK_PROXIES = [
  { id: 'proxy-1', listen_port: 443,  target: 'http://192.168.1.10:3000', hostname: 'grafana.homelab.ts.net',   auto: true,  running: true,  tls_error: '' },
  { id: 'proxy-2', listen_port: 8443, target: 'http://192.168.1.10:9000', hostname: 'portainer.homelab.ts.net', auto: true,  running: true,  tls_error: '' },
  { id: 'proxy-3', listen_port: 7443, target: 'http://192.168.1.30:80',   hostname: 'nginx.homelab.ts.net',     auto: false, running: false, tls_error: '' },
];

const MOCK_TS_STATUS = {
  BackendState: 'Running',
  Hostname: 'tailrelay',
  IPv4: '100.97.110.112',
  MagicDNSName: 'tailrelay.homelab.ts.net.',
  TailnetName: 'homelab.ts.net',
  PeerCount: 4,
  ActivePeers: 2,
  Version: '1.76.6',
  Health: [],
};

const MOCK_TS_PEERS = [
  { DNSName: 'macbook-pro.homelab.ts.net.', IPv4: '100.97.110.50', OS: 'macos',   Active: true,  Online: true,  LastSeen: new Date().toISOString() },
  { DNSName: 'iphone.homelab.ts.net.',      IPv4: '100.97.110.75', OS: 'ios',     Active: false, Online: true,  LastSeen: new Date(Date.now() - 120000).toISOString() },
  { DNSName: 'home-server.homelab.ts.net.', IPv4: '100.97.110.30', OS: 'linux',   Active: true,  Online: true,  LastSeen: new Date().toISOString() },
  { DNSName: 'work-laptop.homelab.ts.net.', IPv4: '100.97.110.90', OS: 'windows', Active: false, Online: false, LastSeen: new Date(Date.now() - 86400000).toISOString() },
];

const MOCK_FUNNELS = [];

const MOCK_BACKUPS = [
  { filename: 'tailrelay-full-2026-03-13T10-00-00.tar.gz', size: 48320, timestamp: '2026-03-13T10:00:00Z', metadata: { backup_type: 'full' } },
  { filename: 'tailrelay-full-2026-03-12T10-00-00.tar.gz', size: 47918, timestamp: '2026-03-12T10:00:00Z', metadata: { backup_type: 'full' } },
  { filename: 'tailrelay-full-2026-03-11T10-00-00.tar.gz', size: 46204, timestamp: '2026-03-11T10:00:00Z', metadata: { backup_type: 'full' } },
];

// Logs: /api/logs returns { logs: [...], level: 'INFO' }
// SSE stream returns text/event-stream with JSON data lines
const MOCK_LOG_ENTRIES = [
  { level: 'INFO',  component: 'serve',     message: 'HTTPS relay started: grafana.homelab.ts.net',        timestamp: new Date(Date.now() - 300000).toISOString() },
  { level: 'INFO',  component: 'serve',     message: 'TCP relay started: plex (32400 → 192.168.1.10:32400)',   timestamp: new Date(Date.now() - 240000).toISOString() },
  { level: 'INFO',  component: 'serve',     message: 'TCP relay started: jellyfin (8096 → 192.168.1.10:8096)', timestamp: new Date(Date.now() - 180000).toISOString() },
  { level: 'WARN',  component: 'tailscale', message: 'Peer work-laptop went offline',                      timestamp: new Date(Date.now() - 120000).toISOString() },
  { level: 'ERROR', component: 'serve',     message: 'Failed to reconcile serve config: connection timeout', timestamp: new Date(Date.now() - 60000).toISOString() },
  { level: 'INFO',  component: 'serve',     message: 'Serve config reconciled successfully',                timestamp: new Date(Date.now() - 30000).toISOString() },
  { level: 'DEBUG', component: 'tailscale', message: 'Polling tailscale status',                            timestamp: new Date().toISOString() },
];

const MOCK_TARGETS = [
  { label: 'Plex',       value: '192.168.1.10:32400' },
  { label: 'Jellyfin',   value: '192.168.1.10:8096'  },
  { label: 'Grafana',    value: '192.168.1.10:3000'  },
  { label: 'Portainer',  value: '192.168.1.10:9000'  },
];

// ── Helpers ──────────────────────────────────────────────────────────────────

async function setupMocks(page, { authenticated = true } = {}) {
  await page.route('**/api/auth/status', r => r.fulfill({ contentType: 'application/json', body: JSON.stringify({ authenticated, needsSetup: false }) }));
  await page.route('**/api/auth/login',  r => r.fulfill({ contentType: 'application/json', body: JSON.stringify({ ok: true }) }));
  await page.route('**/api/serve/tcp/list',    r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_RELAYS) }));
  await page.route('**/api/serve/https/list',  r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_PROXIES) }));
  await page.route('**/api/serve/funnel/list', r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_FUNNELS) }));
  await page.route('**/api/tailscale/status',  r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_TS_STATUS) }));
  await page.route('**/api/tailscale/peers',   r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_TS_PEERS) }));
  await page.route('**/api/backup/list',       r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_BACKUPS) }));
  // /api/logs returns { logs: [...], level: 'INFO' }
  await page.route('**/api/logs',              r => r.fulfill({ contentType: 'application/json', body: JSON.stringify({ logs: MOCK_LOG_ENTRIES, level: 'INFO' }) }));
  await page.route('**/api/logs/level',        r => r.fulfill({ contentType: 'application/json', body: JSON.stringify({ ok: true }) }));
  // SSE stream: serve a valid text/event-stream response with all entries, then a connected ping
  const sseBody = [
    `data: ${JSON.stringify({ connected: true })}\n\n`,
    ...MOCK_LOG_ENTRIES.map(e => `data: ${JSON.stringify(e)}\n\n`),
  ].join('');
  await page.route('**/api/logs/stream', r => r.fulfill({
    contentType: 'text/event-stream',
    headers: { 'Cache-Control': 'no-cache', 'Connection': 'keep-alive' },
    body: sseBody,
  }));
  await page.route('**/api/targets', r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(MOCK_TARGETS) }));
  await page.route('**/api/info',    r => r.fulfill({ contentType: 'application/json', body: JSON.stringify({ version: 'v0.7.0' }) }));
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
  executablePath: `${process.env.HOME}/.cache/ms-playwright/chromium-1208/chrome-linux64/chrome`,
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

  // Log console expanded — use a taller viewport so log area is fully visible
  await page.setViewportSize({ width: 1280, height: 1100 });
  await setTheme(page, 'dark');
  // The LogConsole header button contains "Logs" text — click it to expand
  await page.locator('button', { hasText: 'Logs' }).last().click();
  await page.waitForTimeout(600); // wait for SSE entries and animation
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
  // Click hamburger menu
  const hamburger = page2.locator('button[class*="sm:hidden"]');
  await hamburger.click();
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

  // Navigate to Tailscale tab via JS store
  await page.evaluate(() => {
    // Dispatch a custom event or directly update the store via window
    // Since Svelte stores aren't on window, click the nav button
  });
  // Click the Tailscale nav button
  await page.getByRole('button', { name: /tailscale/i }).first().click();
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

  await page.getByRole('button', { name: /backups/i }).first().click();
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
