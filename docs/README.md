# TideTerm Documentation

This is the home for TideTerm's documentation site. This README is specifically about _building_ and contributing to the docs site.

### Installation

Our docs are built using [Docusaurus](https://docusaurus.io/), a modern static website generator.

### Local Development

```sh
task docsite
```

This command starts a local development server and opens up a browser window. Most changes are reflected live without having to restart the server.

### Build

```sh
task docsite:build:public
```

This command generates static content into the `build` directory and can be served using any static contents hosting service.

### Deployment

Deployments are handled automatically by the [Docsite CI/CD workflow](../.github/workflows/deploy-docsite.yml)
