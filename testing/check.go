// Copyright (c) HashiCorp, Inc.

package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var _ statecheck.StateCheck = checkRemoteImage{}

type checkRemoteImage struct {
	resourceAddress string
}

type manifestPlatform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
}

type manifestItem struct {
	Platform manifestPlatform `json:"platform"`
}

type manifest struct {
	Manifests []manifestItem `json:"manifests"`
}

func (e checkRemoteImage) CheckState(ctx context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	var resource *tfjson.StateResource

	if req.State == nil {
		resp.Error = fmt.Errorf("state is nil")
	}

	if req.State.Values == nil {
		resp.Error = fmt.Errorf("state does not contain any state values")
	}

	if req.State.Values.RootModule == nil {
		resp.Error = fmt.Errorf("state does not contain a root module")
	}

	for _, r := range req.State.Values.RootModule.Resources {
		if e.resourceAddress == r.Address {
			resource = r

			break
		}
	}

	if resource == nil {
		resp.Error = fmt.Errorf("%s - Resource not found in state", e.resourceAddress)

		return
	}
	id, _ := tfjsonpath.Traverse(resource.AttributeValues, tfjsonpath.New("id"))
	platform, _ := tfjsonpath.Traverse(resource.AttributeValues, tfjsonpath.New("platform"))
	repo, _ := name.ParseReference(id.(string))

	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		resp.Error = err
		return
	}

	// Create an ECR client
	svc := ecr.NewFromConfig(cfg)

	input := &ecr.BatchGetImageInput{
		RepositoryName: aws.String(repo.Context().RepositoryStr()),
		ImageIds: []types.ImageIdentifier{
			{
				ImageTag: aws.String(repo.Identifier()),
			},
		},
	}
	if strings.HasPrefix(repo.Identifier(), "sha256:") {
		input.ImageIds = []types.ImageIdentifier{
			{
				ImageDigest: aws.String(repo.Identifier()),
			},
		}
	}
	result, err := svc.BatchGetImage(context.TODO(), input)
	if err != nil {
		resp.Error = fmt.Errorf("failed to get image from ECR: %w", err)
		return
	}
	if len(result.Images) == 0 {
		resp.Error = fmt.Errorf("image %s not found in repository %s", repo.Identifier(), repo.Context().RepositoryStr())
		return
	}

	if platform == nil {
		var m manifest
		err = json.Unmarshal([]byte(*result.Images[0].ImageManifest), &m)
		if err != nil {
			resp.Error = fmt.Errorf("failed to unmarshal image manifest: %w", err)
			return
		}

		var platforms []manifestPlatform
		for _, manifestItem := range m.Manifests {
			if manifestItem.Platform.Architecture != "unknown" && manifestItem.Platform.OS != "unknown" {
				platforms = append(platforms, manifestItem.Platform)
			}
		}
		if len(platforms) < 2 {
			resp.Error = fmt.Errorf("platform not specified, expected image to have multiple platforms: %v", platforms)
			return
		}
	} else {
		p, _ := v1.ParsePlatform(platform.(string))
		opts := []crane.Option{crane.WithPlatform(p)}
		d, _ := crane.Digest(id.(string), opts...)
		if *result.Images[0].ImageId.ImageDigest != d {
			resp.Error = fmt.Errorf("image digest does not match expected digest: %s", *result.Images[0].ImageId.ImageDigest)
			return
		}
	}
}

func CheckRemoteImage(resourceAddress string) statecheck.StateCheck {
	return checkRemoteImage{
		resourceAddress: resourceAddress,
	}
}
