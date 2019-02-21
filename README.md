<p align="center"><br><br>
<img src="https://github.com/linyows/dewy/raw/master/misc/dewy-logo.png" width="150"><br><br><br><br>
</p>

<p align="center">
<strong>DEWY</strong>: The application server for automated deployment with polling a repository.
</p>

<p align="center">
<a href="https://travis-ci.org/linyows/dewy"><img src="https://img.shields.io/travis/linyows/dewy.svg?style=for-the-badge" alt="travis"></a>
<a href="https://github.com/linyows/dewy/releases"><img src="http://img.shields.io/github/release/linyows/dewy.svg?style=for-the-badge" alt="GitHub Release"></a>
<a href="https://github.com/linyows/dewy/blob/master/LICENSE"><img src="http://img.shields.io/badge/license-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
<a href="http://godoc.org/github.com/linyows/dewy"><img src="http://img.shields.io/badge/go-documentation-blue.svg?style=for-the-badge" alt="Go Documentation"></a>
<a href="https://codecov.io/gh/linyows/dewy"> <img src="https://img.shields.io/codecov/c/github/linyows/dewy.svg?style=for-the-badge" alt="codecov"></a>
</p><br><br>

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

![dewy architecture](https://github.com/linyows/dewy/raw/master/misc/dewy-architecture.png)

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

Contribution
------------

1. Fork ([https://github.com/linyows/dewy/fork](https://github.com/linyows/dewy/fork))
1. Create a feature branch
1. Commit your changes
1. Rebase your local changes against the master branch
1. Run test suite with the `go test ./...` command and confirm that it passes
1. Run `gofmt -s`
1. Create a new Pull Request

Author
------

[linyows](https://github.com/linyows)
