/**
 * Split a hostname into a short label plus its domain suffix so the UI can
 * keep the recognisable name + port visible on narrow screens while hiding
 * the (often long) MagicDNS/domain suffix until there is room.
 *
 * `tailrelay.tailnet-domain.ts.net` → { short: 'tailrelay', suffix: '.tailnet-domain.ts.net' }
 * `127.0.0.1`                         → { short: '127.0.0.1',     suffix: '' }   (IPv4 literal kept whole)
 * `myhost`                            → { short: 'myhost',        suffix: '' }   (single label kept whole)
 *
 * Only dotted names whose first label is non-numeric are shortened; this
 * leaves IPv4 literals intact and avoids mangling short hostnames.
 */
export function splitHost(host) {
  if (!host) return { short: '', suffix: '' };
  const dot = host.indexOf('.');
  if (dot > 0 && !/^\d+$/.test(host.slice(0, dot))) {
    return { short: host.slice(0, dot), suffix: host.slice(dot) };
  }
  return { short: host, suffix: '' };
}