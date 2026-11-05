# Basic usage (uses host ~/.docker/config.json if available)
provider "crane" {}

# In-memory authentication with static credentials
provider "crane" {
  alias = "static_auth"

  registry_auth {
    address  = "index.docker.io"
    username = "my-username"
    password = "my-password"
  }
}

# Integration with AWS ECR authorization token
data "aws_ecr_authorization_token" "token" {}

provider "crane" {
  alias = "ecr"

  registry_auth {
    address  = replace(data.aws_ecr_authorization_token.token.proxy_endpoint, "https://", "")
    username = data.aws_ecr_authorization_token.token.user_name
    password = data.aws_ecr_authorization_token.token.password
    # Alternatively using base64 encoded auth:
    # auth   = data.aws_ecr_authorization_token.token.authorization_token
  }
}
