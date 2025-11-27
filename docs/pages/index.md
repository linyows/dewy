---
title: Dewy
description: "Deploy beyond Kubernetes: Zero downtime, zero fuss"
---


{% section .hero %}
{% hero %}
# Deploy beyond Kubernetes: Zero downtime, zero fuss

Dewy is software for declaratively deploying applications in non-Kubernetes environments. It ensures that applications and data on servers are always up-to-date. {% .desc %}

[Get started](/introduction) {% .primarybutton %}
{% /hero %}
{% /section %}

{% section .usecases %}
{% cards %}
- {% item %}
  {% keyword %}
  Server command
  {% /keyword %}
  ## Application Server Auto-Deployment

  ![Dewy Server](/images/dewy-server.png)

  Dewy continuously monitors for new releases and automatically deploys them with graceful restarts, ensuring zero interruption to your running processes.
  Signal-based manual restarts and multi-port configurations are also supported.
  {% /item %}
- {% item %}
  {% keyword %}
  Server command
  {% /keyword %}
  ## Deploy Across Network Boundaries

  ![Dewy Bucket](/images/dewy-bucket.png)

  Upload your artifacts to AWS S3 or Google Cloud Storage, and Dewy handles the rest. Production servers pull updates automatically without needing direct repository access, VPN, or complex firewall rules.
  {% /item %}
- {% item %}
  {% keyword %}
  Container command
  {% /keyword %}
  ## Zero-Downtime Container Deployment

  ![Dewy Container](/images/dewy-container.png)

  Container command enables zero-downtime rolling updates for containerized apps with health checks, automatic traffic switching, and configurable
  replicas—perfect for when you want to deploy everything including the runtime.
  {% /item %}
- {% item %}
  {% keyword %}
  Assets command
  {% /keyword %}
  ## Automated Database Migration

  ![Dewy Assets](/images/dewy-assets.png)

  Deploy schema.sql from your repository and run idempotent migration tools like sqldef via after-deploy hooks. Developers just define the schema, and Dewy automates the entire migration process—freeing your team from manual database operations.
  {% /item %}
{% /cards %}
{% /section %}

{% section .core-benefits %}
{% keyword %}
Core Benefits
{% /keyword %}

![Core Benefits](/images/core-benefits.png)

## Declarative without the Platform
Kubernetes made declarative deployment mainstream, but you shouldn't need a full orchestration platform just to deploy applications declaratively. Dewy delivers Kubernetes-style deployment to simple environments—VPS, VMs, bare metal—without the complexity or overhead.
{% /section %}

{% section .sub-benefits %}
{% keyword %}
Built-in Benefits
{% /keyword %}
- {% item %}
  ![Secure](/images/secure.png)

  ### Secure by Design

  Dewy uses a pull-based architecture where your servers poll registries for updates—no inbound connections required. This eliminates common attack vectors associated with push-based deployments. Combined with audit logging that tracks every deployment, you maintain full visibility and control over who deployed what and when. All registry authentication is handled securely through standard credential management.
  {% /item %}
- {% item %}
  ![Low-cost](/images/low-cost.png)

  ### Low Cost, High Value

  Dewy runs as a single binary with no external dependencies—no complex orchestrators, no expensive infrastructure required. Perfect for VPS, virtual machines, or physical servers, Dewy delivers enterprise-grade deployment automation without the overhead of Kubernetes. Save on infrastructure costs while maintaining professional deployment practices.
  {% /item %}
{% /section %}

{% section .get-started %}
{% sidebyside %}
{% item %}
{% keyword %}
Highly Practical
{% /keyword %}

## 5-Minute Setup, Enterprise-Grade Results

Single binary, zero dependencies, runs anywhere. Setup takes minutes, but you get enterprise-grade deployment automation with audit logs, notifications, and zero-downtime deployments. No complex orchestrators, no steep learning curve—just powerful deployment automation made simple.

[Get started](/getting-started) {% .linkbutton %}
{% /item %}

![Notification and Audit](/images/notifications-audit.png)

{% /sidebyside %}
{% /section %}

{% section .faq %}
## Frequent Questions

{% item %}
### How is Dewy different from Ansible, shell scripts, or CI/CD tools?
Ansible and shell scripts are imperative—you define the steps to execute. Dewy is declarative—you define the desired state, and Dewy continuously maintains it. CI/CD tools (like GitHub Actions) handle build and test, while
Dewy focuses on deployment maintenance. Dewy is also pull-based: servers fetch updates rather than receiving pushes, eliminating SSH access requirements and improving security.
{% /item %}
{% item %}
### Do I need Docker or Kubernetes to use Dewy?
No. Dewy works with bare metal servers, VMs, and VPS without any container platform. The server command deploys binaries directly, and the assets command deploys static files—no Docker required. The container command does require Docker, but it's optional. Dewy is specifically designed for environments where Kubernetes would be overkill.
{% /item %}
{% item %}
### Can I use Dewy for applications written in languages other than Go?
Yes, absolutely. While Dewy itself is written in Go and examples often feature Go applications, you can deploy applications in any language. Use the server command for any compiled binary (Rust, C++, etc.) or interpreted languages (Node.js, Python, Ruby). Use the container command for containerized applications in any language. The only requirement is that your artifacts follow semantic versioning.
{% /item %}
{% item %}
### How do I deploy to multiple servers?
Run Dewy on each server, all pointing to the same registry. Each instance independently polls for updates and deploys the latest version. To avoid registry rate limits, use a shared cache backend like Redis or HashiCorp Consul—this allows multiple Dewy instances to share version information and reduce API calls. All servers automatically converge to the same version.
{% /item %}
{% item %}
### How does Dewy fit into my existing CI/CD pipeline?
Dewy complements your CI/CD pipeline by handling the deployment phase. Your CI/CD (GitHub Actions, GitLab CI, etc.) builds your application and uploads artifacts to a registry (GitHub Releases, S3, GCS, or container registry).

Dewy then monitors that registry and automatically deploys new versions to production. This separation means your CI/CD doesn't need SSH access or deployment credentials to production servers.
{% /item %}
{% item %}
### What happens if a deployment fails?
Dewy includes multiple safety mechanisms. If a before-deploy hook fails, deployment is aborted and the current version continues running. If deployment succeeds but the application fails to start, Dewy logs the error and sends notifications (if configured). The previous 7 releases are kept on disk, allowing for manual rollback by changing the symlink. Dewy also limits error notifications to prevent alert fatigue during persistent failures.
{% /item %}
{% item %}
### Is Dewy free? Is it open source?
Yes, Dewy is completely free and open source under the MIT License. You can use it for personal projects, commercial applications, or enterprise deployments without any cost. The source code is available on [GitHub](https://github.com/linyows/dewy), and contributions are welcome.
{% /item %}
{% /section %}