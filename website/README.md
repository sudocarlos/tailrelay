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
