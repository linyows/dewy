<p align="center">
  <a href="https://dewy.linyo.ws">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://github.com/linyows/dewy/blob/main/misc/dewy-dark-bg.svg?raw=true">
      <img alt="Dewy" src="https://github.com/linyows/dewy/blob/main/misc/dewy.svg?raw=true" width="500">
    </picture>
    <h1 align="center">Dewy<h1>
  </a>
</p>

<p align="center">
  <strong>Dewy</strong> is a Linux service that enables declarative deployment of applications in non-Kubernetes environments.
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

Installation
------------

To install, use `go get`:

```sh
$ go get -d github.com/linyows/dewy
```

Usage
-----

When the application functions as a server:

```sh
$ cd /opt/yourapp
$ env GITHUB_TOKEN=xxx... SLACK_TOKEN=xxx... \
  dewy server --repository yourname/yourapp \
              --artifact yourapp_linux_amd64.tar.gz \
              --port 3000 \
              -- /opt/yourapp/current/yourapp
```

When the application and server are separated, or when the server is unnecessary:

```sh
$ cd /opt/yourapp
$ env GITHUB_TOKEN=xxx... SLACK_TOKEN=xxx... \
  dewy assets --repository yourname/yourapp \
              --artifact yourapp_linux_amd64.tar.gz
```

Architecture
---

Dewy has 3 abstract backends and can be used according to the user's environment.

- Remote repository backend
- Notification backend
- Storage backend

Dewy shares the polling history within the cluster in storage so that it does not communicate excessively to remote repair acquisition.

![dewy architecture](https://github.com/linyows/dewy/raw/main/misc/dewy-architecture.png)

👉 Dewy is not CIOps but GitOps. As in the article on weave works, you do not have to grant permissions externally, so it's simple and easy to solve if problems arise.

Kubernetes anti-patterns: Let's do GitOps, not CIOps!  
https://www.weave.works/blog/kubernetes-anti-patterns-let-s-do-gitops-not-ciops

Server mode
---

Process right after startup:

```sh
$ ps axf
/usr/bin/dewy server ...(main process)
 \_ /opt/your-app/current/your-app --args server (child process)
 ```

When deployment is started, a new child process is created and the old one is gracefully killed.

```sh
$ ps axf
/usr/bin/dewy server ...(main process)
 \_ /opt/your-app/current/your-app --args server (old child process) <-- kill
 \_ /opt/your-app/current/your-app --args server (current child process)
 ```

Provisioning
---

- Chef cookbook - https://github.com/linyows/dewy-cookbook
- Puppet module - https://github.com/takumakume/puppet-dewy

Todo
----

### Repository

- [x] github release
- [ ] git repo

### KVS

- [x] file
- [ ] memory
- [ ] redis
- [ ] consul
- [ ] etcd

### Notification

- [x] slack
- [ ] email

Author
------

[@linyows](https://github.com/linyows)
