# Security policy

## Supported versions

Security fixes go into the latest minor release. Before v1.0.0 that is the newest `v0.x` tag.

## Reporting a vulnerability

Please report security problems privately, through GitHub's private vulnerability reporting: open the repository's
**Security** tab and choose **Report a vulnerability**. Do not open a public issue or pull request for them.

Include what you found, the version or commit it applies to, how to reproduce it, and what an attacker gains. We will
acknowledge the report within a few working days, keep you informed while we work on a fix, and credit you in the
release notes unless you ask us not to.

## Scope

go-obs runs inside your service's HTTP stack, so the issues that matter most are:

- label cardinality an outside client can drive (for example, a path or method reaching a label unnormalized);
- panics, races or leaks in the middleware, the client transport or the response writer wrapper;
- the management server started by `Obs.Serve` exposing more than `/metrics` and the handlers you mounted.

Vulnerabilities in dependencies (Prometheus client, Fiber, gorilla/mux) are best reported upstream; tell us too if
go-obs needs to raise a minimum version.
