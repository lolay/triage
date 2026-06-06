# Security Policy

## Supported versions

triage is on `0.x`. The latest `0.x.y` line receives security fixes; older `0.x`
lines do not. Once `1.0` ships, the policy will be revised to support the latest
stable major and one previous major in line with SemVer expectations.

## Reporting a vulnerability

Please **do not** open a public issue for security problems. Use the private
channel below:

1. **GitHub private vulnerability report (preferred).** Open a draft advisory at
   <https://github.com/lolay/triage/security/advisories/new>. This keeps the
   report visible only to maintainers until a fix and disclosure plan are ready.

In your report, please include:

- A description of the issue and the impact you believe it has.
- Steps to reproduce, or a minimal `triage.yaml` / command line that triggers it.
- The version or commit SHA you tested against (`triage --version`).
- Your operating system.
- Any suggested mitigation, if you have one.

## What to expect

- We aim to acknowledge receipt within **3 business days**.
- We will keep you informed as we investigate, and will coordinate disclosure
  timing with you before publishing a fix or advisory.
- Once a fix lands, we will credit you in the advisory unless you ask us not to.

## Scope

In scope:

- The `triage` binary: config loader, check runners (`tool`/`env`/`path`/
  `one_of`/`command`), the `delegate`/aggregation engine, and output formatters.
- Anything that lets a crafted `triage.yaml` escape the documented read-only,
  no-implicit-shell contract (e.g. a `command` check running outside its declared
  interpreter, or a `delegate` executing something it shouldn't).

Out of scope (please report upstream):

- Vulnerabilities in third-party dependencies unless triage exposes them through
  misuse.
- Issues that require an attacker to already have full local-machine access.
- The behavior of tools triage merely *invokes* (e.g. a bug in `gcloud` itself).
