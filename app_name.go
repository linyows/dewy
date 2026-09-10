package dewy

import "strings"

// appName returns the dewy.app label value that this instance's deploys
// register containers under. Prefers an explicit Config.Container.Name;
// otherwise derives from the registry URL's last path segment.
//
// The fallback used to live inline in three places (lifecycle.go,
// container_deploy.go's deployContainer and stopManagedContainers) and was
// missing from admin_api.go entirely — so /api/containers and /api/status
// would return empty values when --name was omitted even though the deploy
// had created containers under the derived name. Centralizing it here makes
// the deploy / admin / shutdown paths agree by construction.
func (d *Dewy) appName() string {
	if d.config.Container != nil && d.config.Container.Name != "" {
		return d.config.Container.Name
	}
	return deriveAppNameFromRegistry(d.config.Registry)
}

// deriveAppNameFromRegistry pulls the repository segment out of a registry
// URL of the form "<scheme>://<host>/<path>?<query>". The last non-empty
// path component, with any tag (`:`) suffix stripped, is the repository name
// — which is what dewy uses as the default app name. It is scheme-agnostic:
// "img://ghcr.io/owner/app:v1", "ghr://owner/app" and "s3://region/bucket/app"
// all yield "app".
//
// The query is removed before the path is split, because a query value may
// itself contain a "/" that is not a path separator. Trailing empty segments
// are skipped so a URL written with a trailing slash still yields a name.
//
// Returns "" if the URL cannot be parsed enough to find a path component.
func deriveAppNameFromRegistry(registryURL string) string {
	_, rest, found := strings.Cut(registryURL, "://")
	if !found {
		return ""
	}
	rest, _, _ = strings.Cut(rest, "?")

	segments := strings.Split(rest, "/")
	for i := len(segments) - 1; i >= 0; i-- {
		name, _, _ := strings.Cut(segments[i], ":")
		if name != "" {
			return name
		}
	}
	return ""
}
