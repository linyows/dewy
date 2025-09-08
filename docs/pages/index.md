---
title: Dewy
description: Dewy enables declarative deployment of applications in non-Kubernetes environments.
layout: landing
---

Dewy is software primarily designed to declaratively deploy applications written in Go in non-container environments.
Dewy acts as a supervisor for applications, running as the main process while launching the application as a child process.
Its scheduler polls specified registries and, upon detecting the latest version (using semantic versioning), deploys from the designated artifact store.
This enables Dewy to perform pull-based deployments.
Dewy's architecture is composed of abstracted components: registries, artifact stores, cache stores, and notification channels.
Below are diagrams illustrating Dewy's deployment process and architecture.

# Full Next.js example

{% callout %}
This is a full-featured boilerplate for a creating a documentation website using Markdoc and Next.js.
{% /callout %}

## Setup

First, clone this repo and install the dependencies required:

```bash
npm install
# or
yarn install
```

Then, run the development server:

```bash
npm run dev
# or
yarn dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

You can start editing the page by modifying `index.md`. The page auto-updates as you edit the file.
