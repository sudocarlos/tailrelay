# tailrelay documentation site

This is the source for the tailrelay documentation site, built with
[Docusaurus](https://docusaurus.io/). It renders `../docs/openapi.yaml` as
the [API Reference](/api/) via [Redoc](https://redocly.com/redoc) and hosts
guide pages under `docs/`.

## Installation

```bash
npm install
```

## Local Development

```bash
npm start
```

Starts a local dev server with hot reload at `http://localhost:3000`.

## Build

```bash
npm run build
```

Generates static content into the `build/` directory.

## Deployment

Deployment to GitHub Pages is automated by
`.github/workflows/docs.yml` on every push to `main` that touches
`website/**` or `docs/openapi.yaml`. There is no manual deploy step.

## Known deferred vulnerabilities

`image-size` (a transitive dep of `@docusaurus/mdx-loader`) has open
DoS advisories (GitHub alerts #64 and #65) with no upstream fix as of
2.0.2, the latest release. It runs **build-time only** to parse
maintainer-curated screenshots committed to the repo, so there is no
production or untrusted-input exposure. The alerts are dismissed as
"no fix available / tolerable risk"; bump `image-size` to a patched
release (>2.0.2) once upstream ships one.
