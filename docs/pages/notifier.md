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
- **Mail** (`mail://`, `smtp://`): Email notifications via SMTP

## Notification Timing

Dewy sends notifications at the following times:

- **Startup**: Dewy service start
- **Download completion**: New artifact download
- **Deploy success**: Application startup/restart success
- **Error occurrence**: Various error occurrences
- **Hook execution**: Before/After hook execution results
- **Shutdown**: Dewy service shutdown

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
   - `channels:join`: Join channels
   - `chat:write`: Post messages
3. Get token
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
mail://<smtp-host>:<port>/<recipient>
# or
smtp://<smtp-host>:<port>/<recipient>

# Example
dewy server --registry ghr://owner/repo \
  --notifier mail://smtp.gmail.com:587/admin@example.com \
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
  --notifier "mail://smtp.gmail.com:587/admin@example.com?username=sender@gmail.com&password=app-password&from=sender@gmail.com&subject=Deploy+Notification"
```

### Gmail Configuration Example

```bash
# Use environment variables
export MAIL_USERNAME=sender@gmail.com
export MAIL_PASSWORD=your-app-password
export MAIL_FROM=sender@gmail.com

# Execute Dewy
dewy server --registry ghr://owner/repo \
  --notifier "mail://smtp.gmail.com:587/admin@example.com?subject=MyApp+Deploy"
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
   - Are `channels:join` and `chat:write` set in Bot Token Scopes?
   - Is the App installed in the workspace?
3. **Check channel name**
   ```bash
   # For public channels, exclude #
   # ❌ slack://#deployments
   # ✅ slack://deployments

   # For private channels, invite Bot beforehand
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
     --notifier "mail://smtp.example.com:25/admin@example.com?tls=false"
   ```

The notification feature enables sharing deployment status across teams and enables early problem detection and response. Build an efficient operational system with appropriate notification settings.