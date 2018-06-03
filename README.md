DEWY
====

The application server for deployment with distributed polling.

[![Travis](https://img.shields.io/travis/linyows/dewy.svg?style=flat-square)][travis]
[![GitHub release](http://img.shields.io/github/release/linyows/dewy.svg?style=flat-square)][release]
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)][license]
[![Go Documentation](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)][godocs]

[travis]: https://travis-ci.org/linyows/dewy
[release]: https://github.com/linyows/dewy/releases
[license]: https://github.com/linyows/dewy/blob/master/LICENSE
[godocs]: http://godoc.org/github.com/linyows/dewy

Installation
------------

To install, use `go get`:

```sh
$ go get -d github.com/linyows/dewy
```

Useage
------

When the application functions as a server:

```sh
$ dewy server --config /etc/dewy.d/your-application.conf
```

When the application and server are separated, or when the server is unnecessary:

```sh
$ dewy assets --config /etc/dewy.d/your-assets.conf
```

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
