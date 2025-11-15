package provider

import (
	"fmt"
	"testing"

	testutils "github.com/adam-tylr/terraform-provider-crane/testing"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTagsDataSource(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()
	tags := testutils.CopyImagesToRepository(t, repo)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: fmt.Sprintf(testAccTagsDataSourceConfig, repo),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(repo),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("repository"),
						knownvalue.StringExact(repo),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.ListSizeExact(len(tags)),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.SetExact(func() []knownvalue.Check {
							checks := make([]knownvalue.Check, len(tags))
							for i, tag := range tags {
								checks[i] = knownvalue.StringExact(tag)
							}
							return checks
						}()),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("full_ref"),
						knownvalue.Null(),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("omit_digest_tags"),
						knownvalue.Null(),
					),
				},
			},
		},
	})
}

func TestAccTagsDataSourceFullRef(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()
	tags := testutils.CopyImagesToRepository(t, repo)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: fmt.Sprintf(testAccTagsDataSourceConfigFull, repo),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(repo),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("repository"),
						knownvalue.StringExact(repo),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.ListSizeExact(len(tags)),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.SetExact(func() []knownvalue.Check {
							checks := make([]knownvalue.Check, len(tags))
							for i, tag := range tags {
								checks[i] = knownvalue.StringExact(fmt.Sprintf("%s:%s", repo, tag))
							}
							return checks
						}()),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("full_ref"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("omit_digest_tags"),
						knownvalue.Null(),
					),
				},
			},
		},
	})
}

func TestAccTagsDataSourceOmitDigest(t *testing.T) {
	repo, teardown := testutils.CreateRepository(t)
	defer teardown()
	tags := testutils.CopyImagesToRepository(t, repo)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: fmt.Sprintf(testAccTagsDataSourceConfigOmitDigest, repo),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact(repo),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("repository"),
						knownvalue.StringExact(repo),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.ListSizeExact(len(tags)),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.SetExact(func() []knownvalue.Check {
							checks := make([]knownvalue.Check, len(tags))
							for i, tag := range tags {
								checks[i] = knownvalue.StringExact(tag)
							}
							return checks
						}()),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("full_ref"),
						knownvalue.Null(),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("omit_digest_tags"),
						knownvalue.Bool(true),
					),
				},
			},
		},
	})
}

const testAccTagsDataSourceConfig = `
data "crane_tags" "test" {
  repository = "%s"
}
`

const testAccTagsDataSourceConfigFull = `
data "crane_tags" "test" {
  repository = "%s"
  full_ref = true
}
`

const testAccTagsDataSourceConfigOmitDigest = `
data "crane_tags" "test" {
  repository = "%s"
  omit_digest_tags = true
}
`
