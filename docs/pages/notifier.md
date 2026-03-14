---
title: Notifier
description: |
  The notification feature is a Dewy component that automatically communicates deployment status to teams.
  Various events such as success, failure, and hook execution results can be notified via Slack and email.
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## Supported Notification Methods

Dewy supports the following notification methods:

- **Slack** (`slack://`): Notifications to Slack channels
- **Mail** (`smtp://`): Email notifications via SMTP

## Notification Timing

Dewy sends notifications at the following times:

- **Startup**: Dewy service start
- **Download completion**: New artifact download
- **Deploy success**: Application startup/restart success
- **Error occurrence**: Various error occurrences
- **Hook execution**: Before/After hook execution results
- **Shutdown**: Dewy service shutdown

## Quiet Mode

Adding `quiet=true` to the notifier URL suppresses verbose notifications (startup, download, successful hooks) while still delivering important ones (deploy success, errors, hook failures).

```bash
dewy server --registry ghr://owner/repo \
  --notifier "slack://deployments?title=MyApp&quiet=true"
```

## Slack Notifications

Basic configuration

```bash
# Basic format
slack://<channel-name>

# Example
dewy server --registry ghr://owner/repo \
  --notifier slack://deployments \
  -- /opt/myapp/current/myapp
```

Environment variables

```bash
# Slack Bot Token (required)
export SLACK_TOKEN=xoxb-xxxxxxxxxxxxxxxxxxxxx
```

### Slack App Configuration

1. Create Slack App
   - Create app at [https://api.slack.com/apps](https://api.slack.com/apps)
2. Required permissions (Scopes)
   - `chat:write`: Post messages
   - `chat:write.customize`: Customize bot name and icon per message
3. Invite the Slack App to the notification channel
   - You must invite the app to the channel via Slack GUI before sending notifications
4. Get token
   - OAuth & Permissions → Bot User OAuth Token

### Configuration with Options

```bash
# Notification with title
dewy server --registry ghr://owner/repo \
  --notifier "slack://deployments?title=MyApp"

# Notification with URL (link to repository, etc.)
dewy server --registry ghr://owner/repo \
  --notifier "slack://deployments?title=MyApp&url=https://github.com/owner/repo"

# Multiple options
dewy server --registry ghr://owner/repo \
  --notifier "slack://prod-deploy?title=MyApp&url=https://myapp.example.com"
```

### Slack Thread Notifications

When deploying to multiple servers, deploy notifications can flood a Slack channel. Thread notifications group all notifications for the same version into a single Slack thread, keeping the main channel feed clean.

**How it works:**

1. Your CI system (e.g., GitHub Actions) posts a parent message to Slack and saves the message timestamp (`ts`) to a file named `.slack-thread-ts` inside the artifact
2. Dewy extracts the artifact, reads `.slack-thread-ts`, and sends all subsequent notifications as thread replies
3. Error notifications use `reply_broadcast` so they also appear in the main channel feed

**Enable thread mode** by adding `thread=true` to the notifier URL:

```bash
dewy server --registry ghr://owner/repo \
  --notifier "slack://deploy-notify?title=MyApp&url=https://github.com/owner/repo&thread=true" \
  -- /opt/app/current/app
```

**CI-side setup (GitHub Actions + GoReleaser example):**

In your GitHub Actions workflow, post the Slack parent message before running GoReleaser. Then configure GoReleaser to include the `.slack-thread-ts` file in the archive.

`.github/workflows/release.yml`:

```yaml
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Post Slack parent message
        if: startsWith(github.ref, 'refs/tags/')
        env:
          SLACK_TOKEN: ${{ secrets.SLACK_TOKEN }}
          SLACK_CHANNEL: deploy-notify
        run: |
          TAG="${GITHUB_REF#refs/tags/}"
          TS=$(curl -s -X POST https://slack.com/api/chat.postMessage \
            -H "Authorization: Bearer $SLACK_TOKEN" \
            -H "Content-Type: application/json" \
            -d "{\"channel\":\"$SLACK_CHANNEL\",\"text\":\"Release \`$TAG\`\"}" \
            | jq -r '.ts')
          echo "$TS" > .slack-thread-ts

      - uses: goreleaser/goreleaser-action@v6
        with:
          args: release
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

`.goreleaser.yml`:

```yaml
archives:
  - files:
      - .slack-thread-ts
```

{% callout type="important" %}
GoReleaser fails to build if there are uncommitted changes in the working directory. Since `.slack-thread-ts` is generated during CI, you must add it to `.gitignore` to prevent GoReleaser from detecting it as a dirty file.
{% /callout %}

**Behavior summary:**

{% table %}
* Condition
* Behavior
---
* `thread=true` + `.slack-thread-ts` present
* All notifications sent as thread replies. Error notifications are also broadcast to channel.
---
* `thread=true` + `.slack-thread-ts` absent
* Fallback to regular channel posts (same as without thread mode)
---
* `thread=false` (default)
* `.slack-thread-ts` file is ignored even if present. All notifications go to channel.
{% /table %}

### Example Notification Content

```
🚀 Automatic shipping started by Dewy (v1.2.3: server)

✅ Downloaded artifact for v1.2.3

🔄 Server restarted for v1.2.3

❌ Deploy failed: connection timeout
```

## Email Notifications

Basic configuration

```bash
# Basic format
smtp://<smtp-host>:<port>/<recipient>

# Example
dewy server --registry ghr://owner/repo \
  --notifier smtp://smtp.gmail.com:587/admin@example.com \
  -- /opt/myapp/current/myapp
```

Environment variables

```bash
# SMTP authentication info
export MAIL_USERNAME=sender@gmail.com
export MAIL_PASSWORD=app-specific-password
export MAIL_FROM=sender@gmail.com
```

### Configuration Options

{% table %}
* Option
* Type
* Description
* Default Value
---
* `username`
* string
* SMTP authentication username
* MAIL_USERNAME environment variable
---
* `password`
* string
* SMTP authentication password
* MAIL_PASSWORD environment variable
---
* `from`
* string
* Sender address
* MAIL_FROM environment variable or username
---
* `subject`
* string
* Email subject
* "Dewy Notification"
---
* `tls`
* bool
* Use TLS encryption
* true
{% /table %}

### URL Format Configuration

```bash
# Specify all settings with URL parameters
dewy server --registry ghr://owner/repo \
  --notifier "smtp://smtp.gmail.com:587/admin@example.com?username=sender@gmail.com&password=app-password&from=sender@gmail.com&subject=Deploy+Notification"
```

### Gmail Configuration Example

```bash
# Use environment variables
export MAIL_USERNAME=sender@gmail.com
export MAIL_PASSWORD=your-app-password
export MAIL_FROM=sender@gmail.com

# Execute Dewy
dewy server --registry ghr://owner/repo \
  --notifier "smtp://smtp.gmail.com:587/admin@example.com?subject=MyApp+Deploy"
```

{% callout type="important" %}
When using Gmail, you need to enable 2-factor authentication and generate an app password.
Normal Google account passwords cannot be used for authentication.
{% /callout %}

## Error Notification Limits

Dewy limits consecutive error notifications to prevent spam.

- **Limit start**: After 3 consecutive errors, notifications are suppressed
- **Limit release**: Automatically released when normal operation resumes
- **Behavior during limits**: Logs are recorded but notifications are not sent

## Multiple Environment Notification Configuration

### Environment-specific Channels

```bash
# Production environment
dewy server --registry ghr://owner/repo \
  --notifier "slack://prod-deploy?title=MyApp+Production"

# Staging environment
dewy server --registry "ghr://owner/repo?pre-release=true" \
  --notifier "slack://staging-deploy?title=MyApp+Staging"

# Development environment
dewy server --registry "ghr://owner/repo?pre-release=true" \
  --notifier "slack://dev-deploy?title=MyApp+Development"
```

## Troubleshooting

### Slack notifications not arriving

1. **Check token**
   ```bash
   # Test token
   curl -H "Authorization: Bearer $SLACK_TOKEN" \
     https://slack.com/api/auth.test
   ```
2. **Check permissions**
   - Are `chat:write` and `chat:write.customize` set in Bot Token Scopes?
   - Is the App installed in the workspace?
3. **Check channel name**
   ```bash
   # For public channels, exclude #
   # ❌ slack://#deployments
   # ✅ slack://deployments
   ```
4. **Check Bot invitation**
   - Is the Bot invited to the channel? (required for both public and private channels)
   ```

### Email notifications not sending

1. **Check SMTP settings**
   ```bash
   # Test SMTP server connection
   telnet smtp.gmail.com 587
   ```
2. **Check authentication info**
   ```bash
   # Check environment variables
   echo $MAIL_USERNAME
   echo $MAIL_FROM
   # Don't display password
   ```
3. **Check TLS settings**
   ```bash
   # Test with TLS disabled (not recommended)
   dewy server --registry ghr://owner/repo \
     --notifier "smtp://smtp.example.com:25/admin@example.com?tls=false"
   ```

The notification feature enables sharing deployment status across teams and enables early problem detection and response. Build an efficient operational system with appropriate notification settings.
