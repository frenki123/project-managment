---
name: Bug report
about: Something behaves differently than expected
title: ''
labels: bug
assignees: ''
---

## What happened

<!-- Describe the actual behaviour. -->

## What you expected

<!-- Describe the expected behaviour. -->

## Steps to reproduce

1.
2.
3.

## Build

<!-- Both of these matter, because a dev build and a tagged release differ. -->

- Version (`./server --version`):
- Where the binary came from: nightly `dev` build / tagged release / `go run` locally
- Operating system:

## Environment

<!-- The released binary reads these if set. An inherited DEVENV_ROOT or
     DATABASE_URL is enough to make it write somewhere unexpected, so please
     paste whatever you have set. -->

```sh
env | grep -E '^(DEVENV_ROOT|PORT|DATABASE_URL)=' || echo "none set"
```

## Anything else

<!-- Log output, screenshots, or the size of `data/app.db` if the data looks wrong. -->
