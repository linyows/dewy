<p align="right">English | <a href="https://github.com/linyows/dewy/blob/main/README.ja.md">日本語</a></p>

<p align="center">
  <a href="https://dewy.linyo.ws">
    <br><br><br><br><br><br>
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://github.com/linyows/dewy/blob/main/misc/dewy-dark-bg.svg?raw=true">
      <img alt="Dewy" src="https://github.com/linyows/dewy/blob/main/misc/dewy.svg?raw=true" width="240">
    </picture>
    <br><br><br><br><br><br>
  </a>
</p>

<p align="center">
  <strong>Dewy</strong> enables declarative deployment of applications in non-Kubernetes environments.
</p>

<p align="center">
  <a href="https://github.com/linyows/dewy/actions/workflows/build.yml">
    <img alt="GitHub Workflow Status" src="https://img.shields.io/github/actions/workflow/status/linyows/dewy/build.yml?branch=main&style=for-the-badge&labelColor=000000">
  </a>
  <a href="https://github.com/linyows/dewy/releases">
    <img src="http://img.shields.io/github/release/linyows/dewy.svg?style=for-the-badge&labelColor=000000" alt="GitHub Release">
  </a>
  <a href="http://godoc.org/github.com/linyows/dewy">
    <img src="http://img.shields.io/badge/go-documentation-blue.svg?style=for-the-badge&labelColor=000000" alt="Go Documentation">
  </a>
</p>

Dewy is a declarative deployment software designed for Go applications mainly. Dewy acts as a supervisor for applications, running as the main process while launching the application as a child process. Its scheduler polls specified registries and, upon detecting the latest version (using semantic versioning), deploys from the designated artifact store. This enables Dewy to perform pull-based deployments. Dewy’s architecture is composed of abstracted components: registries, artifact stores, cache stores, and notification channels. Below are diagrams illustrating Dewy's deployment process and architecture.

<p align="center">
  <img alt="Dewy Architecture" src="https://github.com/linyows/dewy/blob/main/misc/dewy-architecture.svg?raw=true" width="640"/>
</p>

Features
--

- Pull-based deployment
- Graceful restarts
- Configurable registries and artifact stores
- Deployment status notifications
- Audit logging

Usage
--

The following Server command is an example that uses GitHub Releases as the registry, starts the server on port 8000, sets the log level to info, and sends notifications to Slack.

```sh
$ export GITHUB_TOKEN=****.....
$ export SLACK_TOKEN=****.....
$ dewy server --registry ghr://linyows/dewy-testapp -p 8000 -l info -- /opt/dewy/current/testapp
```

Since Dewy utilizes the GitHub API and Slack API, the relevant environment variables must be set. The registry and notification configurations are formatted similarly to URLs, where the part corresponding to the URL scheme represents the registry or notification type.

```sh
# For a GitHub Releases registry:
--registry ghr://<owner-name>/<repo-name>

# For an AWS S3 registry:
--registry s3://<bucket-name>/<object-prefix>
```

Commands
--

Dewy provides two main commands: `Server` and `Assets`. The `Server` command is designed for server applications, managing the application’s processes and ensuring the application version stays up to date. The `Assets` command focuses on static files such as HTML, CSS, and JavaScript, keeping these assets updated to the latest version.

Interfaces
--

Dewy provides several interfaces, each with selectable implementations. Here’s an overview of each interface.

Interface | Description
---       | ---
Registry  | The registry interface manages versions of applications and files. Current implementations of the registry interface include GitHub Releases, AWS S3, and gRPC. With gRPC, you can build a custom server that satisfies the interface, allowing you to use an existing API as a registry.
Artifact  | The artifact interface manages the actual applications or files. Implementations for artifacts include GitHub Releases, AWS S3, and Google Cloud Storage.
Cache     | The cache interface is used by Dewy to store current versions and artifacts. Available cache implementations include the file system, memory, HashiCorp Consul, and Redis.
Notify    | The Notify are handled through the notification interface, which communicates the deployment status. The available implementation for notifications is Slack.

If additional implementations are needed for any interface, please create an issue.

Semantic Versioning
--

Dewy uses semantic versioning to determine the recency of artifact versions. Therefore, it’s essential to manage software versions using semantic versioning.

Staging
--

Semantic versioning includes a concept called pre-release. A pre-release version is created by appending a suffix with a hyphen to the version number. In a staging environment, adding the option `pre-release=true` to the registry settings enables deployment of pre-release versions.

Background
--

Go can compile code into a single binary tailored for each environment. In distributed systems with orchestrators like Kubernetes, deploying Go applications is typically straightforward. However, for single physical hosts or virtual machine environments without containers, there's no clear answer on how best to deploy a Go binary. Options range from writing shell scripts to use `scp` or `rsync` manually, to using server configuration tools like Ansible or even Ruby-based tools like Capistrano. However, when it comes to managing deployments across teams with audit and visibility into who deployed what, there seems to be a gap in tools that meet these specific needs.

Author
--

[@linyows](https://github.com/linyows)
