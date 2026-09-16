# tailrelay documentation site

This is the source for the tailrelay documentation site, built with
[Material for MkDocs](https://squidfunk.github.io/mkdocs-material/). It
renders `../docs/openapi.yaml` as the [API Reference](docs/api.md) via the
[OpenAPI Docs plugin](https://github.com/Neoteroi/mkdocs-plugins) and hosts
guide pages under `docs/`.

## Installation

```bash
pip install -r requirements.txt
```

## Local Development

```bash
mkdocs serve
```

Starts a local dev server with hot reload at `http://localhost:8000`.

## Build

```bash
mkdocs build --strict
```

Generates static content into the `site/` directory. Strict mode fails the
build on broken links, matching the old Docusaurus `onBrokenLinks: throw`.

## Deployment

Deployment to GitHub Pages is automated by
`.github/workflows/docs.yml` on every push to `main` that touches
`website/**` or `docs/openapi.yaml`. There is no manual deploy step.
