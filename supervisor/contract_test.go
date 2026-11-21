package supervisor

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// contractFile holds the DEWY_PORT format shared with the sock module, which
// applications use to read the variable. The sock tests read the same file,
// so a change to the format on either side fails one of the two test suites.
var contractFile = filepath.Join("..", "sock", "testdata", "dewy_port.json")

type contract struct {
	Port []struct {
		Name  string   `json:"name"`
		Addrs []string `json:"addrs"`
		Env   string   `json:"env"`
	} `json:"port"`
	Health []struct {
		Name string `json:"name"`
		Addr string `json:"addr"`
		FD   int    `json:"fd"`
		Env  string `json:"env"`
	} `json:"health"`
}

func loadContract(t *testing.T) contract {
	t.Helper()
	b, err := os.ReadFile(contractFile)
	if err != nil {
		t.Fatal(err)
	}
	var c contract
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestContractListenersEnv(t *testing.T) {
	for _, tt := range loadContract(t).Port {
		t.Run(tt.Name, func(t *testing.T) {
			lns := make([]*listener, len(tt.Addrs))
			for i, a := range tt.Addrs {
				lns[i] = &listener{addr: a}
			}
			if got := listenersEnv(lns); got != tt.Env {
				t.Errorf("listenersEnv(%q) = %q, want %q", tt.Addrs, got, tt.Env)
			}
		})
	}
}

func TestContractHealthEnv(t *testing.T) {
	for _, tt := range loadContract(t).Health {
		t.Run(tt.Name, func(t *testing.T) {
			addr, err := net.ResolveTCPAddr("tcp", tt.Addr)
			if err != nil {
				t.Fatal(err)
			}
			if got := healthEnv(addr, tt.FD); got != tt.Env {
				t.Errorf("healthEnv(%s, %d) = %q, want %q", addr, tt.FD, got, tt.Env)
			}
		})
	}
}
