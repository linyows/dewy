---
title: Deployment Hooks
description: |
  Deployment hooks are functionality for executing custom commands before and after deployment.
  They enable flexible customization of the deployment process including database backups, service management, and notification sending.
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## Overview {% #overview-details %}

Deployment hooks are powerful functionality for customizing Dewy's automated deployment process. They can execute arbitrary shell commands before and after application deployment, enabling various use cases such as database operations, external service integration, and validation processing.

**Key Features:**
- **Flexible execution timing**: Execution control before and after deployment
- **Complete environment access**: Full access to environment variables and file system
- **Detailed execution results**: Recording of stdout/stderr, exit codes, and execution time
- **Notification integration**: Sending execution results to configured notification channels

## Hook Types and Behavior {% #hook-types %}

### Before Deploy Hook {% #before-hook %}

Hook executed **before** deployment starts.

**Execution Timing:**
- After artifact download
- Before file extraction and symbolic link creation
- Before application restart

**Important Behavior:**
```bash
# If Before Hook fails, deployment is aborted
dewy server --registry ghr://owner/repo \
  --before-deploy-hook "scripts/pre-check.sh" \
  -- /opt/myapp/current/myapp
```

{% callout type="warning" title="Deployment Abort Condition" %}
If Before Deploy Hook exits with a non-zero exit code, the entire deployment process is aborted.
This behavior enables safe deployment prevention when preconditions are not met.
{% /callout %}

### After Deploy Hook {% #after-hook %}

Hook executed after deployment **succeeds**.

**Execution Timing:**
- After file extraction and symbolic link creation completion
- After application restart completion (for server command)
- Final stage of deployment process

**Important Behavior:**
```bash
# Deployment is treated as successful even if After Hook fails
dewy server --registry ghr://owner/repo \
  --after-deploy-hook "scripts/post-deploy-validation.sh" \
  -- /opt/myapp/current/myapp
```

{% callout type="note" %}
After Deploy Hook failure does not affect deployment success status.
However, errors are logged and notifications are sent if configured.
{% /callout %}

## Execution Environment and Constraints {% #execution-environment %}

### Execution Environment {% #environment %}

Hooks are executed in the following environment:

**Shell Execution:**
```bash
/bin/sh -c "your-command"
```

**Working Directory:**
- Dewy's execution directory (usually application root directory)

**Environment Variables:**
- Inherits all environment variables from Dewy process
- Full access to runtime environment variables

### Execution Result Capture {% #execution-results %}

The following information is recorded during hook execution:

{% table %}
* Item
* Description
* Usage
---
* Command
* Executed command string
* Debugging and logging
---
* Stdout
* Standard output content
* Checking execution results
---
* Stderr
* Standard error output
* Understanding error content
---
* ExitCode
* Process exit code
* Success/failure determination
---
* Duration
* Execution time
* Performance monitoring
{% /table %}

**Log Output Example:**
```json
{
  "time": "2024-03-15T10:30:45Z",
  "level": "INFO",
  "msg": "Execute hook success",
  "command": "backup-database.sh",
  "stdout": "Backup completed successfully",
  "stderr": "",
  "exit_code": 0,
  "duration": "2.5s"
}
```

## Configuration Methods {% #configuration %}

### Command Line Configuration {% #command-line %}

```bash
# Basic format
dewy server --registry <registry-url> \
  --before-deploy-hook "<command>" \
  --after-deploy-hook "<command>" \
  -- <application-command>
```

### Configuration Examples {% #configuration-examples %}

**Simple command execution:**
```bash
dewy server --registry ghr://owner/repo \
  --before-deploy-hook "echo 'Starting deployment'" \
  --after-deploy-hook "echo 'Deployment completed'" \
  -- /opt/myapp/current/myapp
```

**Multiple command coordination:**
```bash
dewy server --registry ghr://owner/repo \
  --before-deploy-hook "systemctl stop nginx && backup-db.sh" \
  --after-deploy-hook "systemctl start nginx && send-notification.sh" \
  -- /opt/myapp/current/myapp
```

**Script file execution:**
```bash
dewy server --registry ghr://owner/repo \
  --before-deploy-hook "/opt/scripts/pre-deploy.sh" \
  --after-deploy-hook "/opt/scripts/post-deploy.sh" \
  -- /opt/myapp/current/myapp
```

## Practical Use Cases {% #use-cases %}

### Database Operations {% #database-operations %}

**Automatic backup execution:**
```bash
# PostgreSQL backup
--before-deploy-hook "pg_dump myapp_db > /backup/myapp_$(date +%Y%m%d_%H%M%S).sql"

# MySQL backup
--before-deploy-hook "mysqldump -u root -p myapp_db > /backup/myapp_$(date +%Y%m%d_%H%M%S).sql"
```

**Migration execution:**
```bash
# Rails migration
--after-deploy-hook "cd /opt/myapp/current && bundle exec rake db:migrate"

# Django migration
--after-deploy-hook "cd /opt/myapp/current && python manage.py migrate"

# Go migration (using migrate tool)
--after-deploy-hook "migrate -path /opt/myapp/current/migrations -database 'postgres://...' up"
```

### Service Management {% #service-management %}

**Related service control:**
```bash
# Nginx temporary stop and restart
--before-deploy-hook "systemctl stop nginx"
--after-deploy-hook "systemctl start nginx && systemctl reload nginx"

# Load balancer disconnection
--before-deploy-hook "curl -X DELETE http://lb:8080/servers/$(hostname)"
--after-deploy-hook "curl -X POST http://lb:8080/servers/$(hostname)"
```

**Health check execution:**
```bash
# Application startup confirmation
--after-deploy-hook "timeout 30 bash -c 'until curl -f http://localhost:8080/health; do sleep 1; done'"

# Database connection confirmation
--before-deploy-hook "pg_isready -h localhost -p 5432 -d myapp_db"
```

### Notification and Monitoring {% #notification-monitoring %}

**External system notifications:**
```bash
# Send deployment event to Datadog
--after-deploy-hook "curl -X POST https://api.datadoghq.com/api/v1/events \
  -H 'DD-API-KEY: ${DD_API_KEY}' \
  -d '{\"title\":\"Deployment\",\"text\":\"App deployed\"}'"

# PagerDuty notification
--after-deploy-hook "scripts/notify-pagerduty.sh deployment-success"
```

**Metrics collection:**
```bash
# Record deployment time
--before-deploy-hook "echo $(date +%s) > /tmp/deploy_start"
--after-deploy-hook "echo 'Deploy time: '$(($(date +%s) - $(cat /tmp/deploy_start)))'s'"
```

## Error Handling and Recovery {% #error-handling %}

### Before Hook Failure {% #before-hook-failure %}

Behavior when Before Hook fails:

1. **Automatic deployment abort**: Entire process stops
2. **Error log recording**: Detailed execution results output to logs
3. **Notification sending**: Error notifications sent if configured
4. **Current state maintenance**: Existing application is unaffected

**Recommended recovery procedure:**
```bash
# 1. Check error cause
tail -f /var/log/dewy.log

# 2. Manual hook execution test
/bin/sh -c "your-before-hook-command"

# 3. Retry deployment after problem fix
# Dewy automatically retries on next polling
```

### After Hook Failure {% #after-hook-failure %}

Behavior when After Hook fails:

1. **Deployment processed as successful**: Application runs with new version
2. **Error log recording**: Detailed failure content logged
3. **Notification sending**: Warning notifications sent if configured
4. **Manual response recommended**: Administrator confirmation and response needed

## Security Considerations {% #security %}

### Permission Management {% #permission-management %}

**Execution user permission settings:**
```bash
# Run Dewy with dedicated user
sudo useradd -r -s /bin/bash dewy
sudo chown -R dewy:dewy /opt/myapp

# Service definition with minimum required permissions
# /etc/systemd/system/dewy.service
[Service]
User=dewy
Group=dewy
```

**Sudo usage considerations:**
```bash
# ❌ Dangerous: Password prompt hangs
--before-deploy-hook "sudo systemctl stop nginx"

# ✅ Safe: NOPASSWD or dedicated user permission settings
--before-deploy-hook "systemctl stop nginx"  # systemd user session
```

### Command Injection Prevention {% #injection-prevention %}

**Safe command writing:**
```bash
# ✅ Safe: Protection with quotes
--before-deploy-hook "backup-db.sh --name 'myapp_backup'"

# ❌ Dangerous: Direct environment variable expansion
--before-deploy-hook "echo $USER_INPUT"

# ✅ Safe: Proper environment variable usage
--before-deploy-hook "scripts/safe-command.sh"  # Handle properly in script
```

## Best Practices {% #best-practices %}

### Production Environment Recommended Settings {% #production-settings %}

**Safe backup strategy:**
```bash
dewy server --registry ghr://company/myapp \
  --before-deploy-hook "scripts/production-backup.sh" \
  --after-deploy-hook "scripts/production-validation.sh" \
  --notifier "slack://ops-alerts" \
  -- /opt/myapp/current/myapp
```

**Example production-backup.sh:**
```bash
#!/bin/bash
set -euo pipefail

# Database backup
pg_dump myapp_production > "/backup/pre-deploy-$(date +%Y%m%d_%H%M%S).sql"

# Configuration file backup
cp /opt/myapp/current/config.yml "/backup/config-$(date +%Y%m%d_%H%M%S).yml"

# Health check
curl -f http://localhost:8080/health || exit 1

echo "Pre-deployment backup completed successfully"
```

## Troubleshooting {% #troubleshooting %}

### Common Issues {% #common-issues %}

**Permission errors:**
```bash
# Problem: Permission denied
--before-deploy-hook "systemctl stop nginx"

# Solution: Check and adjust user permissions
sudo usermod -a -G sudo dewy
# or
sudo visudo  # Configure NOPASSWD
```

**Path issues:**
```bash
# Problem: command not found
--after-deploy-hook "npm install"

# Solution: Full path or PATH configuration
--after-deploy-hook "/usr/local/bin/npm install"
# or
--after-deploy-hook "PATH=/usr/local/bin:$PATH npm install"
```

### Debugging Methods {% #debugging %}

**Gradual problem isolation:**
```bash
# 1. Start with simple commands
--before-deploy-hook "echo 'Hook test'"

# 2. Gradually increase complexity
--before-deploy-hook "echo 'Hook test' && date"

# 3. Actual command
--before-deploy-hook "your-actual-command"
```

**Manual execution verification:**
```bash
# Test in same environment as Dewy
cd /opt/myapp
sudo -u dewy /bin/sh -c "your-hook-command"
```

## Related Topics {% #related %}

- [Architecture](/architecture) - Position of hooks in overall deployment process
- [Notifier](/notifier) - Detailed notification channel configuration (Slack/Mail)
- [Versioning](/versioning) - Version detection as deployment trigger
- [FAQ](/faq) - Frequently asked questions about deployment hooks