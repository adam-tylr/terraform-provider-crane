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
  source_digest = data.crane_digest.mutable_tag.digest
  destination   = "my-registry.local/nginx:stable"
}

data "crane_digest" "mutable_tag" {
  reference = "nginx:latest"
}
