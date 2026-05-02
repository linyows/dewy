package dewy

import "fmt"

// proxyBackendUpdater adapts *Dewy to the container.BackendUpdater interface
// so the rolling deploy can register/unregister TCP-proxy backends as new
// replicas come up and old replicas are taken down.
//
// The type alias on *Dewy (rather than a wrapper struct) keeps zero indirection
// and avoids capturing closures: passing (*proxyBackendUpdater)(d) to Deploy
// is the entire glue.
type proxyBackendUpdater Dewy

func (p *proxyBackendUpdater) AddBackend(host string, mappedPort, proxyPort int) error {
	return (*Dewy)(p).addProxyBackend(host, mappedPort, proxyPort)
}

func (p *proxyBackendUpdater) RemoveBackend(host string, mappedPort, proxyPort int) error {
	return (*Dewy)(p).removeProxyBackend(host, mappedPort, proxyPort)
}

// addProxyBackend adds a new backend to the appropriate TCP proxy.
func (d *Dewy) addProxyBackend(host string, port int, proxyPort int) error {
	d.proxyMutex.RLock()
	proxy, exists := d.tcpProxies[proxyPort]
	d.proxyMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no proxy configured for port %d", proxyPort)
	}

	proxy.addBackend(host, port)
	return nil
}

// removeProxyBackend removes a backend from the appropriate TCP proxy.
func (d *Dewy) removeProxyBackend(host string, port int, proxyPort int) error {
	d.proxyMutex.RLock()
	proxy, exists := d.tcpProxies[proxyPort]
	d.proxyMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no proxy configured for port %d", proxyPort)
	}

	if !proxy.removeBackend(host, port) {
		return fmt.Errorf("backend %s:%d not found for proxy port %d", host, port, proxyPort)
	}
	return nil
}
