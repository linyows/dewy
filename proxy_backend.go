package dewy

import "fmt"

// proxyBackendUpdater adapts *Dewy to the container.BackendUpdater interface
// so the rolling deploy can register/unregister TCP-proxy backends as new
// replicas come up and old replicas are taken down.
//
// proxyBackendUpdater is a defined type whose underlying type is Dewy, not
// an alias (which would be `type proxyBackendUpdater = Dewy`). The defined
// form gives us a separate method set scoped to the BackendUpdater contract
// while sharing memory with *Dewy, so the explicit conversion
// (*proxyBackendUpdater)(d) is zero-cost: no wrapper struct, no captured
// closures, just a pointer reinterpretation.
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
