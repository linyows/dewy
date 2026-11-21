package sock

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// contract is the DEWY_PORT format shared with dewy's supervisor package,
// which writes the variable. The supervisor tests read the same file, so a
// change to the format on either side fails one of the two test suites.
type contract struct {
	Port []struct {
		Name    string   `json:"name"`
		Addrs   []string `json:"addrs"`
		Env     string   `json:"env"`
		Entries []struct {
			Addr string  `json:"addr"`
			FD   uintptr `json:"fd"`
		} `json:"entries"`
	} `json:"port"`
}

func loadContract(t *testing.T) contract {
	t.Helper()
	b, err := os.ReadFile("testdata/dewy_port.json")
	if err != nil {
		t.Fatal(err)
	}
	var c contract
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestContractEntries(t *testing.T) {
	for _, tt := range loadContract(t).Port {
		t.Run(tt.Name, func(t *testing.T) {
			t.Setenv(EnvPort, tt.Env)
			got, err := Entries()
			if err != nil {
				t.Fatalf("Entries() error = %v", err)
			}
			want := make([]Entry, len(tt.Entries))
			for i, e := range tt.Entries {
				want[i] = Entry{Addr: e.Addr, FD: e.FD}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Entries() = %+v, want %+v", got, want)
			}
		})
	}
}
