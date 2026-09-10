// Package ociauth resolves the credentials dewy presents to an OCI registry.
//
// Two code paths talk to the same registry with the same credentials: the
// registry package reads the tag list over the registry API (HTTP basic auth,
// exchanged for a bearer token), and the container package pulls the image
// through the container runtime (docker login). Both call Credentials so an
// operator's environment is interpreted the same way on either path — they
// used to read the same variables in a different order, which could leave a
// deployment where the tag list resolved but the pull did not.
package ociauth

import (
	"os"
	"strings"
)

// Credentials returns the username and password to authenticate against
// registry, which is a registry host such as "ghcr.io" or
// "123456789.dkr.ecr.us-east-1.amazonaws.com". Both are empty when the
// environment carries nothing usable, which callers treat as an anonymous
// request.
//
// Resolution order:
//
//  1. DOCKER_USERNAME / DOCKER_PASSWORD, when a username is set. Setting
//     these is an explicit choice, so it wins over anything inferred from
//     the registry host.
//  2. GITHUB_TOKEN for ghcr.io.
//  3. AWS_ECR_PASSWORD for ECR.
//  4. GCR_TOKEN for Google Artifact Registry and Container Registry.
//  5. Whatever DOCKER_PASSWORD holds, with an empty username. Callers that
//     require both parts treat this as no credentials.
func Credentials(registry string) (username, password string) {
	if u := os.Getenv("DOCKER_USERNAME"); u != "" {
		return u, os.Getenv("DOCKER_PASSWORD")
	}

	// GitHub Container Registry
	if strings.Contains(registry, "ghcr.io") {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			return "token", token
		}
	}

	// AWS ECR. The password is the token half of the `AWS:<token>` pair that
	// `aws ecr get-login-password` prints.
	if strings.Contains(registry, ".ecr.") && strings.Contains(registry, ".amazonaws.com") {
		if token := os.Getenv("AWS_ECR_PASSWORD"); token != "" {
			return "AWS", token
		}
	}

	// Google Artifact Registry / Container Registry. The password is the
	// service account key JSON.
	if strings.Contains(registry, "gcr.io") || strings.Contains(registry, "-docker.pkg.dev") {
		if token := os.Getenv("GCR_TOKEN"); token != "" {
			return "_json_key", token
		}
	}

	return "", os.Getenv("DOCKER_PASSWORD")
}
