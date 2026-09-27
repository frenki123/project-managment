# Security Policy

## Reporting a vulnerability

Please do not open a public issue for a security problem.

Use **Security → Report a vulnerability** on this repository to file a private
report. That opens a private advisory visible only to you and the maintainer,
and it leaves a public record once a fix ships.

## What to include

- The build you tested, and where you got it: `./server --version`, plus whether
  it was the nightly `dev` build or a tagged release
- How to reproduce
- Any relevant environment: the released binary reads `DEVENV_ROOT`, `PORT` and
  `DATABASE_URL` if they are set, so include those

## Supported versions

Only the most recent build is supported. This project is pre-release and has no
long-term-support branch, so there is no older version to patch — upgrade to the
current build.

## Scope

In scope: the server, the `/api/v1` endpoints, and the SQLite schema.

Out of scope: anything requiring an attacker to already have write access to the
repository or your machine.

## Response

This is a solo-maintained pre-release project, so treat the response as best
effort rather than a commitment. The practical path is usually: acknowledge the
report, agree on a fix, ship it, and credit you in the release notes unless you
would rather stay anonymous.
