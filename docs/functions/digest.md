---
page_title: "crane::digest Function - terraform-provider-crane"
description: |-
  Resolve the content digest for a container image reference using crane.
---

# crane::digest (Function)

Resolve the content digest for a container image reference using [`crane`](https://github.com/google/go-containerregistry).

## Example Usage

```terraform
output "digest" {
  value = crane::digest("registry.example.com/team/app:latest")
}
```

## Arguments Reference

- `reference` (String) A tag or digest identifying the image to inspect (for example `registry/repository:tag`).

## Return Value

- `String` Content digest of the referenced image, such as `sha256:...`.
