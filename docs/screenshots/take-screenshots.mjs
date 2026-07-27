/**
 * Screenshot capture script for tailrelay UI.
 * Uses Playwright with mocked API responses — no backend required.
 * Requires Vite dev server running: cd webui/frontend && npm run dev
 * Run: node take-screenshots.mjs
 *
 * Each capture is written here and mirrored into the Docusaurus static dir so
 * the published docs site never drifts from the sources committed under docs/.
 */
import { chromium } from 'playwright';
import { copyFile } from 'fs/promises';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dir = dirname(fileURLToPath(import.meta.url));
const OUT = __dir;
const SITE_OUT = resolve(__dir, '../../website/static/img/screenshots');
const BASE = 'http://localhost:5173';

// ── Mock data ────────────────────────────────────────────────────────────────

const FQDN = 'tailrelay.homelab.ts.net';

// /api/serve/tcp/list → [{ relay: ServeRelay, running: bool }]
const MOCK_TCP_RELAYS = [
  { relay: { id: 'tcp-32400', type: 'tcp', listen_port: 32400, target_host: '192.168.1.10', target_port: 32400, enabled: true,  autostart: true  }, running: true  },
  { relay: { id: 'tcp-25565', type: 'tcp', listen_port: 25565, target_host: '192.168.1.20', target_port: 25565, enabled: false, autostart: false }, running: false },
  { relay: { id: 'tcp-8096',  type: 'tcp', listen_port: 8096,  target_host: '192.168.1.10', target_port: 8096,  enabled: true,  autostart: true  }, running: true  },
];

// /api/serve/https/list → [ServeRelay + { hostname, running, listener_scheme }]
const MOCK_HTTPS_RELAYS = [
  { id: 'https-443',  type: 'https', listen_port: 443,  target_host: '192.168.1.10', target_port: 3000, target_https: false, enabled: true,  autostart: true,  hostname: FQDN, running: true,  listener_scheme: 'https' },
  { id: 'https-8443', type: 'https', listen_port: 8443, target_host: '192.168.1.10', target_port: 9000, target_https: false, enabled: true,  autostart: true,  hostname: FQDN, running: true,  listener_scheme: 'https' },
  { id: 'https-7443', type: 'https', listen_port: 7443, target_host: '192.168.1.30', target_port: 80,   target_https: false, enabled: false, autostart: false, hostname: FQDN, running: false, listener_scheme: 'https' },
];

// /api/serve/funnel/list → [ServeRelay + { hostname, running }]
const MOCK_FUNNEL_RELAYS = [
  { id: 'funnel-10000', type: 'funnel', funnel_transport: 'https', listen_port: 10000, target_host: '192.168.1.10', target_port: 8080, target_https: false, enabled: true, autostart: true, hostname: FQDN, running: true },
];

// /api/tailscale/status → tailscale.StatusSummary (Go field names, no json tags)
const MOCK_TS_STATUS = {
  Connected: true,
  BackendState: 'Running',
  Hostname: 'tailrelay',
  MagicDNSName: FQDN,
  IPv4: '100.97.110.112',
  IPv6: 'fd7a:115c:a1e0::5a01:6e70',
  TailnetName: 'homelab.ts.net',
  Version: '1.90.8',
  PeerCount: 7,
  ActivePeers: 5,
  Health: [],
  LastCheck: new Date().toISOString(),
  IsCustomControlServer: false,
  ControlServer: '',
};

// /api/tailscale/peers → []tailscale.PeerInfo (Go field names, no json tags)
const MOCK_TS_PEERS = [
  { Hostname: 'macbook-pro', DNSName: 'macbook-pro.homelab.ts.net', OS: 'macos',   IPv4: '100.97.110.50', TailscaleIPs: ['100.97.110.50'], Active: true,  Online: true,  LastSeen: new Date().toISOString(),                        Relay: 'nyc', ExitNode: false },
  { Hostname: 'iphone',      DNSName: 'iphone.homelab.ts.net',      OS: 'ios',     IPv4: '100.97.110.75', TailscaleIPs: ['100.97.110.75'], Active: false, Online: true,  LastSeen: new Date(Date.now() - 120000).toISOString(),     Relay: 'nyc', ExitNode: false },
  { Hostname: 'home-server', DNSName: 'home-server.homelab.ts.net', OS: 'linux',   IPv4: '100.97.110.30', TailscaleIPs: ['100.97.110.30'], Active: true,  Online: true,  LastSeen: new Date().toISOString(),                        Relay: 'nyc', ExitNode: true  },
  { Hostname: 'proxmox-ve',  DNSName: 'proxmox-ve.homelab.ts.net',  OS: 'linux',   IPv4: '100.97.110.20', TailscaleIPs: ['100.97.110.20'], Active: true,  Online: true,  LastSeen: new Date().toISOString(),                        Relay: 'nyc', ExitNode: false },
  { Hostname: 'nas-storage', DNSName: 'nas-storage.homelab.ts.net', OS: 'freebsd', IPv4: '100.97.110.25', TailscaleIPs: ['100.97.110.25'], Active: true,  Online: true,  LastSeen: new Date().toISOString(),                        Relay: 'nyc', ExitNode: false },
  { Hostname: 'ipad',        DNSName: 'ipad.homelab.ts.net',        OS: 'ios',     IPv4: '100.97.110.76', TailscaleIPs: ['100.97.110.76'], Active: false, Online: false, LastSeen: new Date(Date.now() - 3600000).toISOString(),    Relay: 'chi', ExitNode: false },
  { Hostname: 'work-laptop', DNSName: 'work-laptop.homelab.ts.net', OS: 'windows', IPv4: '100.97.110.90', TailscaleIPs: ['100.97.110.90'], Active: false, Online: false, LastSeen: new Date(Date.now() - 86400000).toISOString(),   Relay: 'chi', ExitNode: false },
];

// /api/tailscale/networking → tailscale.NetworkingSummary (Go field names)
const MOCK_TS_NETWORKING = {
  AdvertiseExitNode: false,
  ExitNodeAllowLANAccess: true,
  AdvertiseRoutes: ['192.168.1.0/24'],
  AcceptRoutes: true,
  ExitNode: '',
  SSH: false,
};

// /api/backup/list → []config.BackupInfo
const MOCK_BACKUPS = [
  { filename: 'tailrelay-full-2026-03-13T10-00-00.tar.gz', size: 48320, timestamp: '2026-03-13T10:00:00Z', metadata: { timestamp: '2026-03-13T10:00:00Z', version: 'v0.9.6', hostname: 'tailrelay', backup_type: 'full' } },
  { filename: 'tailrelay-full-2026-03-12T10-00-00.tar.gz', size: 47918, timestamp: '2026-03-12T10:00:00Z', metadata: { timestamp: '2026-03-12T10:00:00Z', version: 'v0.9.6', hostname: 'tailrelay', backup_type: 'full' } },
  { filename: 'tailrelay-cfg-2026-03-11T10-00-00.tar.gz',  size: 6204,  timestamp: '2026-03-11T10:00:00Z', metadata: { timestamp: '2026-03-11T10:00:00Z', version: 'v0.9.5', hostname: 'tailrelay', backup_type: 'config-only' } },
];

// /api/logs → { logs: []logger.Entry, level }; /api/logs/stream → SSE of the same entries
const MOCK_LOG_ENTRIES = [
  { level: 'INFO',  source: 'serve',     message: 'HTTPS relay started: 443 → 192.168.1.10:3000',        timestamp: new Date(Date.now() - 300000).toISOString() },
  { level: 'INFO',  source: 'serve',     message: 'TCP relay started: 32400 → 192.168.1.10:32400',       timestamp: new Date(Date.now() - 240000).toISOString() },
  { level: 'INFO',  source: 'serve',     message: 'TCP relay started: 8096 → 192.168.1.10:8096',         timestamp: new Date(Date.now() - 180000).toISOString() },
  { level: 'WARN',  source: 'tailscale', message: 'Peer work-laptop went offline',                       timestamp: new Date(Date.now() - 120000).toISOString() },
  { level: 'ERROR', source: 'serve',     message: 'Failed to reconcile serve config: connection timeout', timestamp: new Date(Date.now() - 60000).toISOString() },
  { level: 'INFO',  source: 'serve',     message: 'Serve config reconciled successfully',                 timestamp: new Date(Date.now() - 30000).toISOString() },
  { level: 'DEBUG', source: 'tailscale', message: 'Polling tailscale status',                             timestamp: new Date().toISOString() },
];

// /api/targets → targets.json entries used to populate the "Preset Target" select
const MOCK_TARGETS = [
  { target_name: 'Plex',      app_id: 'plex',      host: '192.168.1.10', port: 32400, type: 'tcp',   protocol: 'tcp'   },
  { target_name: 'Jellyfin',  app_id: 'jellyfin',  host: '192.168.1.10', port: 8096,  type: 'tcp',   protocol: 'tcp'   },
  { target_name: 'Grafana',   app_id: 'grafana',   host: '192.168.1.10', port: 3000,  type: 'https', protocol: 'http'  },
  { target_name: 'Portainer', app_id: 'portainer', host: '192.168.1.10', port: 9000,  type: 'https', protocol: 'https' },
];

// ── Helpers ──────────────────────────────────────────────────────────────────

const json = (body) => ({ contentType: 'application/json', body: JSON.stringify(body) });

async function setupMocks(page, { authenticated = true } = {}) {
  page.on('pageerror', err => console.error('PAGE ERROR:', err.message));

  // Catch-all first: Playwright matches most-recently-registered routes first,
  // so the specific handlers below take precedence. Without this fallback an
  // unmocked endpoint hits a real backend, and a 401 there logs the app out
  // mid-capture — silently yielding login screenshots.
  await page.route('**/api/**', r => {
    console.warn(`  unmocked API request: ${r.request().url()}`);
    return r.fulfill(json({}));
  });

  await page.route('**/api/auth/status',          r => r.fulfill(json({ authenticated, needsSetup: false })));
  await page.route('**/api/auth/login',           r => r.fulfill(json({ ok: true })));
  await page.route('**/api/serve/tcp/list',       r => r.fulfill(json(MOCK_TCP_RELAYS)));
  await page.route('**/api/serve/https/list',     r => r.fulfill(json(MOCK_HTTPS_RELAYS)));
  await page.route('**/api/serve/funnel/list',    r => r.fulfill(json(MOCK_FUNNEL_RELAYS)));
  await page.route('**/api/tailscale/status',     r => r.fulfill(json(MOCK_TS_STATUS)));
  await page.route('**/api/tailscale/peers',      r => r.fulfill(json(MOCK_TS_PEERS)));
  await page.route('**/api/tailscale/networking', r => r.fulfill(json(MOCK_TS_NETWORKING)));
  await page.route('**/api/tailscale/control-server', r => r.fulfill(json({ control_server: '' })));
  await page.route('**/api/backup/list',          r => r.fulfill(json(MOCK_BACKUPS)));
  await page.route('**/api/logs',                 r => r.fulfill(json({ logs: MOCK_LOG_ENTRIES, level: 'INFO' })));
  await page.route('**/api/logs/level',           r => r.fulfill(json({ level: 'INFO' })));
  // SSE stream stays open with no entries; history from /api/logs is enough
  // and replaying it here would duplicate every line in the console.
  await page.route('**/api/logs/stream',          r => r.fulfill({
    contentType: 'text/event-stream',
    headers: { 'Cache-Control': 'no-cache', Connection: 'keep-alive' },
    body: '',
  }));
  await page.route('**/api/targets',              r => r.fulfill(json(MOCK_TARGETS)));
  await page.route('**/api/info',                 r => r.fulfill(json({ version: 'v0.9.6', commit: 'abc1234' })));
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
  await copyFile(path, resolve(SITE_OUT, `${name}.png`));
  console.log(`  saved ${name}.png`);
}

// ── Main ─────────────────────────────────────────────────────────────────────

const browser = await chromium.launch({
  executablePath: chromium.executablePath(),
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

  // Log console expanded — taller viewport so the whole console fits in frame
  await setTheme(page, 'dark');
  await page.setViewportSize({ width: 1280, height: 1100 });
  const logToggle = page.locator('button').filter({ hasText: /^\s*Logs/ }).last();
  await logToggle.click();
  await page.waitForTimeout(500);
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
  await page.waitForTimeout(300);
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
