// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	testutils "github.com/adam-tylr/terraform-provider-crane/testing"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccDigestDataSource(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()

	tags := testutils.CopyImagesToRepository(t, repo)
	imageRef := fmt.Sprintf("%s:%s", repo, tags[0])

	expectedDigest, err := crane.Digest(imageRef)
	if err != nil {
		t.Fatalf("failed to read digest for %s: %v", imageRef, err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testAccDigestDataSourceConfig, imageRef),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.crane_digest.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(imageRef),
					),
					statecheck.ExpectKnownValue(
						"data.crane_digest.test",
						tfjsonpath.New("reference"),
						knownvalue.StringExact(imageRef),
					),
					statecheck.ExpectKnownValue(
						"data.crane_digest.test",
						tfjsonpath.New("digest"),
						knownvalue.StringExact(expectedDigest),
					),
				},
			},
		},
	})
}

const testAccDigestDataSourceConfig = `
data "crane_digest" "test" {
  reference = "%s"
}
`
