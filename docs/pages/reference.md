---
title: Dewy CLI Reference
description: Complete reference guide for Dewy CLI commands and options
---

# Dewy CLI Reference

This page provides detailed information about Dewy CLI commands, options, environment variables, and usage examples.

## Basic Commands

Dewy provides two main commands: `server` and `assets`. These commands are used to deploy and manage applications.

### server Command

The `dewy server` command starts the main Dewy process and handles application deployment and monitoring. This command provides the core functionality of Dewy.

```bash
dewy server [options] -- [application command]
```

### assets Command

The `dewy assets` command displays detailed information about the current artifacts. This is useful for checking deployment status.

```bash
dewy assets [options]
```

## Command Line Options

Use the following options to customize Dewy's behavior.

### --registry (-r)

Specifies the registry URL to retrieve application version information. Supports various registries including GitHub Releases, DockerHub, and ECR.

```bash
dewy server --registry ghr://owner/repo -- /opt/app/current/app
```

### --artifact (-a)

Specifies the location of artifacts to download. Supports multiple protocols including S3, GitHub, and HTTP/HTTPS.

```bash
dewy server --artifact s3://bucket/path/to/artifact -- /opt/app/current/app
```

### --cache (-c)

Specifies artifact cache settings. Local filesystem or Redis can be used as cache storage.

```bash
dewy server --cache file:///tmp/dewy-cache -- /opt/app/current/app
```

### --notifier (-n)

Configures deployment status notifications. Notification channels such as Slack, Discord, and email can be set up.

```bash
dewy server --notifier slack://webhook-url -- /opt/app/current/app
```

### --port (-p)

Specifies the port used by Dewy's HTTP server. Default is 8080.

```bash
dewy server --port 9090 -- /opt/app/current/app
```

### --interval (-i)

Specifies the interval in seconds to check the registry. Default is 600 seconds (10 minutes).

```bash
dewy server --interval 300 -- /opt/app/current/app
```

### --keeptime (-k)

Specifies the time in seconds to retain artifacts. Old artifacts are automatically deleted.

```bash
dewy server --keeptime 86400 -- /opt/app/current/app
```

### --timezone (-t)

Specifies the timezone used for logs and scheduling. Default is UTC.

```bash
dewy server --timezone Asia/Tokyo -- /opt/app/current/app
```

### --user (-u)

Specifies the user to run the application. Running with a dedicated user is recommended for security reasons.

```bash
dewy server --user app-user -- /opt/app/current/app
```

### --group (-g)

Specifies the group to run the application. Use in combination with the user option.

```bash
dewy server --user app-user --group app-group -- /opt/app/current/app
```

### --workdir (-w)

Specifies the working directory for the application. This becomes the base directory when the application reads and writes files.

```bash
dewy server --workdir /opt/app/data -- /opt/app/current/app
```

### --verbose (-v)

Enables verbose log output. Useful for debugging and troubleshooting.

```bash
dewy server --verbose -- /opt/app/current/app
```

### --version

Displays Dewy version information.

```bash
dewy --version
```

### --help (-h)

Displays help for available commands and options.

```bash
dewy --help
dewy server --help
```

## Environment Variables

Dewy can use the following environment variables to customize behavior. Environment variables have lower priority than command line options.

### DEWY_REGISTRY

Sets the default registry URL. Has the same effect as the `--registry` option.

```bash
export DEWY_REGISTRY=ghr://owner/repo
```

### DEWY_ARTIFACT

Sets the default artifact URL. Has the same effect as the `--artifact` option.

```bash
export DEWY_ARTIFACT=s3://bucket/path/to/artifact
```

### DEWY_CACHE

Specifies the default cache settings. Has the same effect as the `--cache` option.

```bash
export DEWY_CACHE=file:///tmp/dewy-cache
```

### DEWY_NOTIFIER

Specifies the default notification settings. Has the same effect as the `--notifier` option.

```bash
export DEWY_NOTIFIER=slack://webhook-url
```

### DEWY_PORT

Sets the Dewy HTTP server port. Has the same effect as the `--port` option.

```bash
export DEWY_PORT=8080
```

### DEWY_INTERVAL

Sets the registry check interval. Has the same effect as the `--interval` option.

```bash
export DEWY_INTERVAL=600
```

### DEWY_KEEPTIME

Sets the artifact retention time. Has the same effect as the `--keeptime` option.

```bash
export DEWY_KEEPTIME=86400
```

### DEWY_TIMEZONE

Sets the timezone. Has the same effect as the `--timezone` option.

```bash
export DEWY_TIMEZONE=Asia/Tokyo
```

### DEWY_USER

Sets the execution user. Has the same effect as the `--user` option.

```bash
export DEWY_USER=app-user
```

### DEWY_GROUP

Sets the execution group. Has the same effect as the `--group` option.

```bash
export DEWY_GROUP=app-group
```

### DEWY_WORKDIR

Sets the working directory. Has the same effect as the `--workdir` option.

```bash
export DEWY_WORKDIR=/opt/app/data
```

## Registry URL Formats

Dewy supports multiple registry types, each using different URL formats.

### GitHub Releases (ghr://)

Used to retrieve version information from GitHub Releases. Supports both public and private repositories.

```bash
ghr://owner/repository
ghr://owner/repository#tag-pattern
```

### Docker Hub (dockerhub://)

Retrieves version information from Docker Hub image tags. Can also be used with containerized applications.

```bash
dockerhub://namespace/repository
dockerhub://namespace/repository:tag-pattern
```

### Amazon ECR (ecr://)

Retrieves version information from Amazon Elastic Container Registry. AWS credentials are required.

```bash
ecr://region/repository
ecr://account-id.dkr.ecr.region.amazonaws.com/repository
```

### Git (git://)

Retrieves version information from Git repository tags. Supports SSH and HTTPS authentication.

```bash
git://github.com/owner/repository
git://gitlab.com/owner/repository
```

## Notification Formats

Dewy supports various notification channels. Deployment success and failure can be notified to appropriate destinations.

### Slack

Sends notifications using Slack Incoming Webhook or Bot Token. Channel specification is also possible.

```bash
slack://webhook-url
slack://token@channel
```

### Discord

Sends notifications using Discord Webhook or Bot Token.

```bash
discord://webhook-url
discord://token@channel-id
```

### Microsoft Teams

Sends notifications using Microsoft Teams Incoming Webhook.

```bash
teams://webhook-url
```

### Email (SMTP)

Sends email notifications through SMTP server. Authentication credentials and server settings are required.

```bash
smtp://user:password@host:port/to@example.com
```

### HTTP/HTTPS

POSTs notifications to custom HTTP endpoints. Supports webhook-style notifications.

```bash
http://your-webhook-endpoint
https://your-webhook-endpoint
```

## Exit Codes

Dewy uses the following exit codes to indicate execution results. These can be used for processing branches in scripts and CI/CD pipelines.

### Normal Exit (0)

Returned when the command completes normally. All processes executed as expected.

### Configuration Error (1)

Returned when there are problems with command line options or configuration files. Option verification or configuration review is required.

### Network Error (2)

Returned when connection to registry or artifacts fails. Check network connectivity and authentication credentials.

### Filesystem Error (3)

Returned when file read/write or directory access fails. Check permissions and disk space.

### Application Error (4)

Returned when the launched application terminates abnormally. Check application logs.

## Usage Examples

Here are common usage patterns for Dewy. Adjust settings according to your actual environment.

### Basic Usage Example

The simplest configuration to start Dewy. Monitors GitHub Releases and deploys applications.

```bash
dewy server \
  --registry ghr://owner/repo \
  --port 8080 \
  -- /opt/app/current/myapp
```

### Complete Configuration Example

Comprehensive configuration example specifying all major options. Suitable for production environment use.

```bash
dewy server \
  --registry ghr://mycompany/myapp \
  --artifact s3://mybucket/artifacts/ \
  --cache redis://localhost:6379/0 \
  --notifier slack://hooks.slack.com/services/xxx/yyy/zzz \
  --port 8080 \
  --interval 300 \
  --keeptime 86400 \
  --timezone Asia/Tokyo \
  --user app-user \
  --group app-group \
  --workdir /opt/app/data \
  --verbose \
  -- /opt/app/current/myapp --config /opt/app/config/app.conf
```

### Environment Variables Example

Example using environment variables to keep the command line concise. This approach has good compatibility with Docker environments and configuration management tools.

```bash
export DEWY_REGISTRY=ghr://mycompany/myapp
export DEWY_ARTIFACT=s3://mybucket/artifacts/
export DEWY_CACHE=file:///tmp/dewy-cache
export DEWY_NOTIFIER=slack://hooks.slack.com/services/xxx/yyy/zzz
export DEWY_PORT=8080
export DEWY_INTERVAL=300
export DEWY_TIMEZONE=Asia/Tokyo

dewy server -- /opt/app/current/myapp
```

### Artifact Information Check Example

Example to check the current artifact status. Can be used to understand deployment situation.

```bash
dewy assets --registry ghr://mycompany/myapp --verbose
```

### Development Environment Example

Configuration example for checking at short intervals in development environment. Suitable for environments requiring frequent updates.

```bash
dewy server \
  --registry ghr://mycompany/myapp-dev \
  --interval 60 \
  --port 8080 \
  --verbose \
  -- /opt/app/dev/myapp --env development
```