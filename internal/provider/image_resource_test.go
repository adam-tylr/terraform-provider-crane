// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	testutils "github.com/adam-tylr/terraform-provider-crane/testing"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccImageResourceRemoteImage(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()

	sourceAlpine := testutils.CreateSourceRef("docker/library/alpine")
	sourceAlpine3 := testutils.CreateSourceRef("docker/library/alpine:3")

	if err := testutils.SeedMockMultiArchImage(t, sourceAlpine, []string{"linux/amd64", "linux/arm64"}); err != nil {
		t.Fatalf("failed to seed mock source image: %v", err)
	}
	if err := testutils.SeedMockMultiArchImage(t, sourceAlpine3, []string{"linux/amd64", "linux/arm64"}); err != nil {
		t.Fatalf("failed to seed mock source image tag: %v", err)
	}

	alpineDigest, err := crane.Digest(sourceAlpine)
	if err != nil {
		t.Fatalf("failed to read source digest: %v", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccImage(sourceAlpine, fmt.Sprintf("%s:latest", repo)),
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
						knownvalue.StringExact(sourceAlpine),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
			// Update Source tag
			{
				Config: testAccImage(sourceAlpine3, fmt.Sprintf("%s:latest", repo)),
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
						knownvalue.StringExact(sourceAlpine3),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
			// Update Source digest
			{
				Config: testAccImage(fmt.Sprintf("%s@%s", sourceAlpine, alpineDigest), fmt.Sprintf("%s:latest", repo)),
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
						knownvalue.StringExact(fmt.Sprintf("%s@%s", sourceAlpine, alpineDigest)),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
			// Update Destination tag
			{
				Config: testAccImage(sourceAlpine3, fmt.Sprintf("%s:3", repo)),
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
						knownvalue.StringExact(sourceAlpine3),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:3", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionReplace,
						),
					},
				},
			},
		},
	})
}

func TestAccImageResourceSourceDigest(t *testing.T) {
	sourceRepo, teardownSource := testutils.CreateRepository(t)
	defer teardownSource()
	destinationRepo, teardownDestination := testutils.CreateRepository(t)
	defer teardownDestination()

	sourceLatest := fmt.Sprintf("%s:latest", sourceRepo)
	sourceUpdate := fmt.Sprintf("%s:update", sourceRepo)
	destination := fmt.Sprintf("%s:latest", destinationRepo)

	if err := testutils.SeedMockMultiArchImage(t, sourceLatest, []string{"linux/amd64", "linux/arm64"}); err != nil {
		t.Fatalf("failed to seed source latest image: %v", err)
	}
	if err := testutils.SeedMockMultiArchImage(t, sourceUpdate, []string{"linux/amd64", "linux/arm64"}); err != nil {
		t.Fatalf("failed to seed source update image: %v", err)
	}

	sourceDigest, err := crane.Digest(sourceLatest)
	if err != nil {
		t.Fatalf("failed to read source digest: %v", err)
	}
	updatedSourceDigest, err := crane.Digest(sourceUpdate)
	if err != nil {
		t.Fatalf("failed to read updated source digest: %v", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccImageWithSourceDigest(sourceLatest, destination, fmt.Sprintf("\"%s\"", sourceDigest)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("digest"),
						knownvalue.StringExact(sourceDigest),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
			{
				PreConfig: func() {
					if err := crane.Copy(sourceUpdate, sourceLatest); err != nil {
						t.Fatalf("failed to update source latest image: %v", err)
					}
				},
				Config: testAccImageWithSourceDigest(sourceLatest, destination, fmt.Sprintf("\"%s\"", updatedSourceDigest)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("digest"),
						knownvalue.StringExact(updatedSourceDigest),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
		},
	})
}

func TestAccImageResourceWithTarball(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()
	tarPath := testutils.CreateLocalTarball(t, testutils.CreateSourceRef("docker/library/alpine:latest"))
	updatedTarPath := testutils.CreateLocalTarball(t, testutils.CreateSourceRef("nginx/nginx:latest"))
	defer os.Remove(tarPath)
	defer os.Remove(updatedTarPath)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccImageWithPlatform(tarPath, fmt.Sprintf("%s:latest", repo), "linux/amd64"),
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
						knownvalue.StringExact(tarPath),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
			// Update Source path
			{
				Config: testAccImageWithPlatform(updatedTarPath, fmt.Sprintf("%s:latest", repo), "linux/amd64"),
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
						knownvalue.StringExact(updatedTarPath),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
		},
	})
}

func TestAccImageResourceWithPlatform(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()

	sourceAlpineLatest := testutils.CreateSourceRef("docker/library/alpine:latest")
	sourceAlpine := testutils.CreateSourceRef("docker/library/alpine")

	platforms := []string{"linux/amd64", "linux/arm64"}
	if err := testutils.SeedMockMultiArchImage(t, sourceAlpineLatest, platforms); err != nil {
		t.Fatalf("failed to seed mock multi-arch source: %v", err)
	}
	if err := testutils.SeedMockMultiArchImage(t, sourceAlpine, platforms); err != nil {
		t.Fatalf("failed to seed mock multi-arch source tag: %v", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccImageWithPlatform(sourceAlpineLatest, fmt.Sprintf("%s:latest", repo), "linux/amd64"),
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
						knownvalue.StringExact(testutils.CreateSourceRef("docker/library/alpine:latest")),
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
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
			// Update the platform to a different architecture
			{
				Config: testAccImageWithPlatform(sourceAlpine, fmt.Sprintf("%s:latest", repo), "linux/arm64"),
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
						knownvalue.StringExact(sourceAlpine),
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
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
			// Nullify the platform
			{
				Config: testAccImage(sourceAlpine, fmt.Sprintf("%s:latest", repo)),
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
						knownvalue.StringExact(sourceAlpine),
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
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
		},
	})
}

func TestAccImageResourceExternalDeletion(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()

	sourceAlpine := testutils.CreateSourceRef("docker/library/alpine")
	if err := testutils.SeedMockMultiArchImage(t, sourceAlpine, []string{"linux/amd64", "linux/arm64"}); err != nil {
		t.Fatalf("failed to seed mock source image: %v", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccImage(sourceAlpine, fmt.Sprintf("%s:latest", repo)),
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
						knownvalue.StringExact(sourceAlpine),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionCreate,
						),
					},
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
	defer teardown()

	sourceAlpine := testutils.CreateSourceRef("docker/library/alpine")
	if err := testutils.SeedMockMultiArchImage(t, sourceAlpine, []string{"linux/amd64", "linux/arm64"}); err != nil {
		t.Fatalf("failed to seed mock source image: %v", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccImage(sourceAlpine, fmt.Sprintf("%s:latest", repo)),
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
						knownvalue.StringExact(sourceAlpine),
					),
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("destination"),
						knownvalue.StringExact(fmt.Sprintf("%s:latest", repo)),
					),
					testutils.CheckRemoteImage("crane_image.test"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"crane_image.test",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
			{
				// Delete the image externally and expect the resource to be recreated
				PreConfig: func() {
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

func TestAccImageResourceImport(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()

	sourceNginx := testutils.CreateSourceRef("nginx/nginx:latest")
	if err := testutils.SeedMockImage(t, sourceNginx); err != nil {
		t.Fatalf("failed to seed mock source image: %v", err)
	}

	tags := testutils.CopyImagesToRepository(t, repo)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccImage(testutils.CreateSourceRef(fmt.Sprintf("nginx/nginx:%s", tags[0])), fmt.Sprintf("%s:%s", repo, tags[0])),
			},
			{
				ResourceName:            "crane_image.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source"},
			},
		},
	})
}

func TestAccImageResourceSourceRepositoryDoesNotExist(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()

	missingSource := testutils.CreateSourceRef(fmt.Sprintf("missing/%s:latest", strings.ToLower(t.Name())))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccImage(missingSource, fmt.Sprintf("%s:latest", repo)),
				ExpectError: regexp.MustCompile("Error reading source image"),
			},
		},
	})
}

func TestAccImageResourceDestinationRepositoryDoesNotExist(t *testing.T) {
	sourceAlpineLatest := testutils.CreateSourceRef("docker/library/alpine:latest")
	if err := testutils.SeedMockImage(t, sourceAlpineLatest); err != nil {
		t.Fatalf("failed to seed mock source image: %v", err)
	}

	missingRepo := fmt.Sprintf("localhost:9999/missing-%s", strings.ToLower(t.Name()))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccImage(
					sourceAlpineLatest,
					fmt.Sprintf("%s:latest", missingRepo),
				),
				ExpectError: regexp.MustCompile("Error pushing image to destination"),
			},
		},
	})
}

func TestAccImageResourceDestinationTagAlreadyExistsWithDifferentDigest(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()

	destination := fmt.Sprintf("%s:latest", repo)
	if err := testutils.SeedMockImage(t, destination); err != nil {
		t.Fatalf("failed to seed repository with initial image: %v", err)
	}

	sourceNginx := testutils.CreateSourceRef("nginx/nginx:latest")
	if err := testutils.SeedMockImage(t, sourceNginx); err != nil {
		t.Fatalf("failed to seed mock source image: %v", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccImage(sourceNginx, destination),
				ExpectError: regexp.MustCompile("Destination image already exists but does not match source"),
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

func testAccImageWithSourceDigest(source string, destination string, sourceDigest string) string {
	return fmt.Sprintf(`
resource "crane_image" "test" {
  source = %q
  destination = %q
  source_digest = %s
}
`, source, destination, sourceDigest)
}
