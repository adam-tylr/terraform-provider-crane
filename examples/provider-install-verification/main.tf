# Copyright (c) HashiCorp, Inc.

terraform {
  required_providers {
    crane = {
      source = "adam-tylr/crane"
    }
  }
}

provider "crane" {}