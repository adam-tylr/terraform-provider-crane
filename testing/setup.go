// Copyright (c) HashiCorp, Inc.

package testing

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/google/go-containerregistry/pkg/crane"
)

func CreateRepository(t *testing.T) (string, func()) {
	t.Helper()

	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		t.Fatalf("failed to load AWS configuration: %v", err)
	}

	// Create an ECR client
	svc := ecr.NewFromConfig(cfg)

	// Create a new repository
	repoName := "test-repo-" + strings.ToLower(t.Name())
	t.Logf("Creating repository: %s", repoName)
	input := &ecr.CreateRepositoryInput{
		RepositoryName: aws.String(repoName),
	}

	response, err := svc.CreateRepository(context.TODO(), input)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	return *response.Repository.RepositoryUri, func() {
		// Cleanup: delete the repository after the test
		deleteInput := &ecr.DeleteRepositoryInput{
			RepositoryName: aws.String(repoName),
			Force:          true,
		}
		_, err := svc.DeleteRepository(context.TODO(), deleteInput)
		if err != nil {
			t.Logf("failed to delete repository: %v", err)
		}
	}
}

func CopyImagesToRepository(t *testing.T, targetRepoUri string) (tags []string) {
	t.Helper()

	imageTags := []string{"latest", "alpine"}
	cwd, _ := os.Getwd()
	index := strings.Index(cwd, "terraform-provider-crane")
	root := cwd[:index+len("terraform-provider-crane")]
	testingDir := path.Join(root, "testing")

	for _, tag := range imageTags {
		src := fmt.Sprintf("nginx:%s", tag)
		dst := fmt.Sprintf("%s:%s", targetRepoUri, tag)

		tarPath := path.Join(testingDir, fmt.Sprintf("%s.tar.gz", tag))
		// Avoid dockerhub rate limits by saving and loading images from a tarball
		if _, err := os.Stat(tarPath); os.IsNotExist(err) {
			t.Logf("Pulling image '%s' to save to %s", src, tarPath)
			img, err := crane.Pull(src)
			if err != nil {
				t.Fatalf("failed to pull image %s: %v", src, err)
			}
			err = crane.Save(img, tag, tarPath)
			if err != nil {
				t.Fatalf("failed to save image %s to %s: %v", src, tarPath, err)
			}
		}

		t.Logf("Pushing image '%s' to '%s'", src, dst)
		img, err := crane.Load(tarPath)
		if err != nil {
			t.Fatalf("failed to load image from %s: %v", tarPath, err)
		}
		err = crane.Push(img, dst)
		if err != nil {
			t.Fatalf("failed to push image from %s to %s: %v", tarPath, dst, err)
		}
	}
	return imageTags
}

func DeleteRemoteImage(t *testing.T, repoUri string, tag string) {
	t.Helper()

	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		t.Fatalf("failed to load AWS configuration: %v", err)
	}

	// Create an ECR client
	svc := ecr.NewFromConfig(cfg)

	// Delete the image from the repository
	input := &ecr.BatchDeleteImageInput{
		RepositoryName: aws.String(strings.Split(repoUri, "/")[1]),
		ImageIds: []types.ImageIdentifier{
			{ImageTag: aws.String(tag)},
		},
	}

	_, err = svc.BatchDeleteImage(context.TODO(), input)
	if err != nil {
		t.Fatalf("failed to delete image %s from repository %s: %v", tag, repoUri, err)
	}
}
