resource "crane_image" "example" {
  source      = "alpine:3.22.2"
  destination = "my-registry.local/alpine:3.22.2"
}

resource "crane_image" "from_file" {
  source      = "path/to/local/image-1.2.3.tar"
  destination = "my-registry.local/my-image:latest"
}

# When source is a mutable tagged remote image, use a crane_digest data source with source_digest to trigger updates
resource "crane_image" "mutable_tag" {
  source        = "nginx:latest"
  source_digest = data.crane_digest.mutable_tag.digest
  destination   = "my-registry.local/nginx:stable"
}

data "crane_digest" "mutable_tag" {
  reference = "nginx:latest"
}

# When source is a mutable file, use filemd5 or filesha256 with source_digest to trigger updates
resource "crane_image" "mutable_file" {
  source        = "path/to/local/image.tar"
  source_digest = filemd5("path/to/local/image.tar")
  destination   = "my-registry.local/nginx:stable"
}
