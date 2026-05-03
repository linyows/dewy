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
// had created containers under the derived name. Centralising it here makes
// the deploy / admin / shutdown paths agree by construction.
func (d *Dewy) appName() string {
	if d.config.Container != nil && d.config.Container.Name != "" {
		return d.config.Container.Name
	}
	return deriveAppNameFromRegistry(d.config.Registry)
}

// deriveAppNameFromRegistry pulls the repository segment out of a registry
// URL of the form "<scheme>://<host>/<path>?<query>". The last path
// component, with any tag (`:`) and query (`?`) suffix stripped, is the
// repository name — which is what dewy uses as the default app name.
//
// Returns "" if the URL cannot be parsed enough to find a path component.
func deriveAppNameFromRegistry(registryURL string) string {
	parts := strings.SplitN(registryURL, "://", 2)
	if len(parts) != 2 {
		return ""
	}
	pathParts := strings.Split(parts[1], "/")
	if len(pathParts) == 0 {
		return ""
	}
	last := pathParts[len(pathParts)-1]
	last = strings.Split(last, "?")[0]
	last = strings.Split(last, ":")[0]
	return last
}
