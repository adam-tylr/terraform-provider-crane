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

var SOURCE_REGISTRY = os.Getenv("SOURCE_REGISTRY")

func CreateSourceRef(image string) string {
	return fmt.Sprintf("%s/%s", SOURCE_REGISTRY, image)
}

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

func CreateLocalTarball(t *testing.T, imageRef string) string {
	t.Helper()

	tag := strings.Split(imageRef, ":")[1]
	// Get the name by taking the part after the last slash and before the colon
	nameParts := strings.Split(imageRef, "/")
	name := nameParts[len(nameParts)-1]
	name = strings.Split(name, ":")[0]

	cwd, _ := os.Getwd()
	index := strings.Index(cwd, "terraform-provider-crane")
	root := cwd[:index+len("terraform-provider-crane")]
	testingDir := path.Join(root, "testing")
	tarPath := path.Join(testingDir, fmt.Sprintf("%s%s.tar.gz", name, tag))

	if _, err := os.Stat(tarPath); os.IsNotExist(err) {
		t.Logf("Creating local tarball: %s", tarPath)

		img, err := crane.Pull(imageRef)
		if err != nil {
			t.Fatalf("failed to pull image %s: %v", imageRef, err)
		}

		err = crane.Save(img, imageRef, tarPath)
		if err != nil {
			t.Fatalf("failed to create tarball for image %s: %v", imageRef, err)
		}
	}

	return tarPath
}

func CopyImagesToRepository(t *testing.T, targetRepoUri string) (tags []string) {
	t.Helper()

	imageTags := []string{"latest", "alpine"}

	for _, tag := range imageTags {
		src := fmt.Sprintf("%s/nginx/nginx:%s", SOURCE_REGISTRY, tag)
		dst := fmt.Sprintf("%s:%s", targetRepoUri, tag)

		t.Logf("Copying image '%s' from '%s'", src, dst)
		err := crane.Copy(src, dst)
		if err != nil {
			t.Fatalf("failed to copy image from %s to %s: %v", src, dst, err)
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
