// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	testutils "github.com/adam-tylr/terraform-provider-crane/testing"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccImageResource(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccImage("alpine", fmt.Sprintf("%s:latest", repo)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("reference"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("alpine"),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
			},
			// Update Source tag
			{
				Config: testAccImage("alpine:3", fmt.Sprintf("%s:latest", repo)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("reference"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("alpine:3"),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
			},
			// Update Source digest
			{
				Config: testAccImage("alpine@sha256:de4fe7064d8f98419ea6b49190df1abbf43450c1702eeb864fe9ced453c1cc5f", fmt.Sprintf("%s:latest", repo)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("reference"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("alpine@sha256:de4fe7064d8f98419ea6b49190df1abbf43450c1702eeb864fe9ced453c1cc5f"),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
			},
			// Update Destination tag
			{
				Config: testAccImage("alpine:3", fmt.Sprintf("%s:3", repo)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(fmt.Sprintf("%s:3", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("reference"),
						knownvalue.StringExact(fmt.Sprintf("%s:3", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("alpine:3"),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:3", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
			},
		},
		// CheckDestroy: testutils.CheckImageDestroy(repo),
	})
}

func TestAccImageResourceWithPlatform(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccImageWithPlatform("alpine:latest", fmt.Sprintf("%s:latest", repo), "linux/amd64"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("reference"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("alpine:latest"),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("platform"),
						knownvalue.StringExact("linux/amd64"),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
			},
			// Update the platform to a different architecture
			{
				Config: testAccImageWithPlatform("alpine", fmt.Sprintf("%s:latest", repo), "linux/arm64"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("reference"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("alpine"),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("platform"),
						knownvalue.StringExact("linux/arm64"),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
			},
			// Nullify the platform
			{
				Config: testAccImage("alpine", fmt.Sprintf("%s:latest", repo)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("reference"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("alpine"),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("platform"),
						knownvalue.Null(),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
			},
		},
	})
}

func TestAccImageResourceExternalDeletion(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccImage("alpine", fmt.Sprintf("%s:latest", repo)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("reference"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("alpine"),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
			},
			{
				// Delete the image externally and expect the resource to be recreated
				PreConfig: func() {
					// Delete the image from the remote repository
					testutils.DeleteRemoteImage(t, repo, "latest")
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
		},
	})
}

func TestAccImageResourceExternalRepoDeletion(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccImage("alpine", fmt.Sprintf("%s:latest", repo)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("reference"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("source"),
						knownvalue.StringExact("alpine"),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
			},
			{
				// Delete the image externally and expect the resource to be recreated
				PreConfig: func() {
					teardown()
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
		},
	})
}

func testAccImage(source string, destination string) string {
	return fmt.Sprintf(`
resource "crane_image" "test" {
  source = %q
  destination = %q
}
`, source, destination)
}

func testAccImageWithPlatform(source string, destination string, platform string) string {
	return fmt.Sprintf(`
resource "crane_image" "test" {
  source = %q
  destination = %q
  platform = %q
}
`, source, destination, platform)
}

// func testAccImageWithSourceDigest(source string, destination string, sourceDigest string) string {
// 	return fmt.Sprintf(`
// resource "crane_image" "test" {
//   source = %q
//   destination = %q
//   source_digest = %q
// }
// `, source, destination, sourceDigest)
// }

// Test Cases for the image resource
// Source is a tar file
// Source is a remote image
// Copy an image with a digest instead of a tag
// Copy an image with a tag
// Copy only one architecture of a multi-arch image
// Trigger update by changing the source digest
// Trigger update by changing the source tag
// Trigger update by changing the destination tag
// Trigger update by changing the architecture of a multi-arch image
// Run an apply with the underlying image no longer existing
// Run an apply with the underlying repo no longer existing
