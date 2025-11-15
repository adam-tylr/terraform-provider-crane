resource "crane_image" "example" {
  source      = "alpine:latest"
  destination = "my-registry.local/alpine:latest"
}

resource "crane_image" "from_file" {
  source      = "path/to/local/image.tar"
  destination = "my-registry.local/my-image:latest"
}

resource "crane_image" "mutable_tag" {
  source        = "nginx:latest"
  source_digest = provider::crane::digest("nginx:latest")
  destination   = "my-registry.local/nginx:stable"
}
