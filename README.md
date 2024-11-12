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

Dewy is software primarily designed to declaratively deploy applications written in Go in non-container environments. Dewy acts as a supervisor for applications, running as the main process while launching the application as a child process. Its scheduler polls specified registries and, upon detecting the latest version (using semantic versioning), deploys from the designated artifact store. This enables Dewy to perform pull-based deployments. Dewy’s architecture is composed of abstracted components: registries, artifact stores, cache stores, and notification channels. Below are diagrams illustrating Dewy's deployment process and architecture.

<p align="center">
  <img alt="Dewy Architecture" src="https://github.com/linyows/dewy/blob/main/misc/dewy-architecture.svg?raw=true" width="640"/>
</p>

Features
--

- Pull-based declaratively deployment
- Graceful restarts
- Configurable registries and artifact stores
- Deployment status notifications
- Audit logging

Usage
--

The following Server command demonstrates how to use GitHub Releases as a registry, start a server on port 8000, set the log level to `info` and enable notifications via Slack.

```sh
$ dewy server --registry ghr://linyows/myapp \
  --notify slack://general?title=myapp -p 8000 -l info -- /opt/myapp/current/myapp
```

The registry and notification configurations are URL-like structures, where the scheme component represents the registry or notification type. More details are provided in the Registry section.

Commands
--

Dewy provides two main commands: `Server` and `Assets`. The `Server` command is designed for server applications, managing the application’s processes and ensuring the application version stays up to date. The `Assets` command focuses on static files such as HTML, CSS, and JavaScript, keeping these assets updated to the latest version.

-	server
-	assets

Interfaces
--

Dewy provides several interfaces, each with multiple implementations to choose from. Below are brief descriptions of each. (Feel free to create an issue if there’s an implementation you’d like added.)

- Registry
- Artifact
- Cache
- Notify

Registry
--

The Registry interface manages versions of applications and files. It currently supports GitHub Releases, AWS S3, and GRPC as sources.

### Common Options

There are two common options for the registry.

Option | Type | Description
---    | ---  | ---  
pre-release | bool | Set to true to include pre-release versions, following semantic versioning.
artifact | string | Specify the artifact filename if it does not follow the name_os_arch.ext pattern that Dewy matches by default.

### Github Releases

To use GitHub Releases as a registry, configure it as follows and set up the required environment variables for accessing the GitHub API.

```sh
# Format
# ghr://<owner-name>/<repo-name>?<options: pre-release, artifact>

# Example
$ export GITHUB_TOKEN=****.....
$ dewy --registry ghr://linyows/myapp?pre-release=true&artifact=dewy.tar ...
```

### AWS S3

To use AWS S3 as a registry, configure it as follows. Options include specifying the region and endpoint (for S3-compatible services). Required AWS API credentials must also be set as environment variables.

```sh
# Format
# s3://<region-name>/<bucket-name>/<path-prefix>?<options: endpoint, pre-release, artifact>

# Example
$ export AWS_ACCESS_KEY_ID=****.....
$ export AWS_SECRET_ACCESS_KEY=****.....
$ dewy --registry s3://jp-north-1/dewy/foo/bar/myapp?endpoint=https://s3.isk01.sakurastorage.jp ...
```

Please ensure that the object path in S3 follows the order: `<prefix>/<semver>/<artifact>`. For example:

```sh
# <path...>/<semver>/<artifact>
foo/bar/baz/v1.2.4-rc/dewy-testapp_linux_x86_64.tar.gz
                   /dewy-testapp_linux_arm64.tar.gz
                   /dewy-testapp_darwin_arm64.tar.gz
foo/bar/baz/v1.2.3/dewy-testapp_linux_x86_64.tar.gz
                  /dewy-testapp_linux_arm64.tar.gz
                  /dewy-testapp_darwin_arm64.tar.gz
foo/bar/baz/v1.2.2/dewy-testapp_linux_x86_64.tar.gz
                  /dewy-testapp_linux_arm64.tar.gz
                  /dewy-testapp_darwin_arm64.tar.gz
```

Dewy leverages aws-sdk-go-v2, so you can also specify region and endpoint through environment variables.

```sh
$ export AWS_ENDPOINT_URL="http://localhost:9000"
```

### GRPC

For using GRPC as a registry, configure as follows. Since the GRPC server defines artifact URLs, options like pre-release and artifact are not available. This registry is suitable if you wish to control artifact URLs or reporting dynamically.

```
# Format
# grpc://<server-host>?<options: no-tls>

# Example
$ dewy --registry grpc://localhost:9000?no-tls=true ...
```

Artifact
--

The Artifact interface manages application or file content itself. If the registry is not GRPC, artifacts will automatically align with the registry type. Supported types include GitHub Releases, AWS S3, and Google Cloud Storage.

Cache
--

The Cache interface stores the current versions and artifacts. Supported implementations include the file system, memory, HashiCorp Consul, and Redis.

Notify
--

The Notify interface sends deployment status updates. Slack and SMTP are available as notification methods.

### Slack

To use Slack for notifications, configure as follows. Options include a title and url that can link to the repository name or URL. You’ll need to [create a Slack App](https://api.slack.com/apps), generate an OAuth Token, and set the required environment variables. The app should have `channels:join` and `chat:write` permissions.

```sh
# Format
# slack://<channel-name>?<options: title, url>

# Example
$ export SLACK_TOKEN=****.....
$ dewy --notify slack://dewy?title=myapp&url=https://dewy.liny.ws ...
```

Semantic Versioning
--

Dewy uses semantic versioning to determine the recency of artifact versions. Therefore, it’s essential to manage software versions using semantic versioning.

```text
# Pre release versions：
v1.2.3-rc
v1.2.3-beta.2
```

For details, visit https://semver.org/

Staging
--

Semantic versioning includes a concept called pre-release. A pre-release version is created by appending a suffix with a hyphen to the version number. In a staging environment, adding the option `pre-release=true` to the registry settings enables deployment of pre-release versions.

Provisioning
--

Provisioning for Dewy is available via Chef and Puppet. Ansible support is not currently available—feel free to contribute if you’re interested.

- Chef: https://github.com/linyows/dewy-cookbook
- Puppet: https://github.com/takumakume/puppet-dewy

Background
--

Go can compile code into a single binary tailored for each environment. In distributed systems with orchestrators like Kubernetes, deploying Go applications is typically straightforward. However, for single physical hosts or virtual machine environments without containers, there's no clear answer on how best to deploy a Go binary. Options range from writing shell scripts to use `scp` or `rsync` manually, to using server configuration tools like Ansible or even Ruby-based tools like Capistrano. However, when it comes to managing deployments across teams with audit and visibility into who deployed what, there seems to be a gap in tools that meet these specific needs.

Frequently Asked Questions
--

Here are some questions you may be asked:

- What happens if I delete the latest version from the registry?
    
    Dewy will adjust to the new latest version after deletion. While it’s generally not recommended to delete or overwrite released versions, there may be cases, such as security concerns, where deletion is unavoidable.
    
- Where can I find the audit logs?
    
    Audit logs are saved as text files at the location where the artifacts are hosted. Currently, there is no search functionality for these logs.
    If we identify a better solution, we may make adjustments. Additionally, it may be necessary to send notifications to observability tools like OTEL for enhanced monitoring, separate from the audit logs.
    
- How can I prevent registry rate limits caused by polling from multiple Dewy instances?
    
    By using a cache component like HashiCorp Consul or Redis, you can share the cache among multiple Dewy instances, which reduces the total number of requests to the registry. In this case, it's best to set an appropriate registry TTL.
    You can also extend the polling interval by specifying it in the command options.

Author
--

[@linyows](https://github.com/linyows)
