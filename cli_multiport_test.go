package dewy

import (
	"reflect"
	"testing"
)

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
		wantErr  bool
	}{
		{
			name:     "empty input",
			input:    []string{},
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "single port",
			input:    []string{"8080"},
			expected: []string{"8080"},
			wantErr:  false,
		},
		{
			name:     "multiple single ports",
			input:    []string{"8080", "8081", "8082"},
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:     "comma-separated ports",
			input:    []string{"8080,8081,8082"},
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:     "port range",
			input:    []string{"8080-8082"},
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:     "mixed formats",
			input:    []string{"8080", "8085,8086", "8090-8092"},
			expected: []string{"8080", "8085", "8086", "8090", "8091", "8092"},
			wantErr:  false,
		},
		{
			name:     "duplicates removed",
			input:    []string{"8080", "8081", "8080"},
			expected: []string{"8080", "8081"},
			wantErr:  false,
		},
		{
			name:     "sorted output",
			input:    []string{"8082", "8080", "8081"},
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:    "invalid port",
			input:   []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "port out of range high",
			input:   []string{"70000"},
			wantErr: true,
		},
		{
			name:    "port out of range low",
			input:   []string{"0"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parsePorts(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePorts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parsePorts() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParsePortSpec(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
		wantErr  bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "single port",
			input:    "8080",
			expected: []string{"8080"},
			wantErr:  false,
		},
		{
			name:     "comma-separated ports",
			input:    "8080,8081,8082",
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:     "comma-separated with spaces",
			input:    "8080, 8081 , 8082",
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:     "port range",
			input:    "8080-8082",
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:     "mixed comma and range",
			input:    "8080,8085-8087,8090",
			expected: []string{"8080", "8085", "8086", "8087", "8090"},
			wantErr:  false,
		},
		{
			name:    "invalid port in comma-separated",
			input:   "8080,invalid,8082",
			wantErr: true,
		},
		{
			name:    "invalid range format",
			input:   "8080-8081-8082",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parsePortSpec(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePortSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parsePortSpec() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParsePortRange(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
		wantErr  bool
	}{
		{
			name:     "valid range",
			input:    "8080-8082",
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:     "single port range",
			input:    "8080-8080",
			expected: []string{"8080"},
			wantErr:  false,
		},
		{
			name:     "range with spaces",
			input:    " 8080 - 8082 ",
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:    "reverse range",
			input:   "8082-8080",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "8080-8081-8082",
			wantErr: true,
		},
		{
			name:    "non-numeric start",
			input:   "abc-8082",
			wantErr: true,
		},
		{
			name:    "non-numeric end",
			input:   "8080-abc",
			wantErr: true,
		},
		{
			name:    "range too large",
			input:   "8000-8200",
			wantErr: true,
		},
		{
			name:    "invalid port in range",
			input:   "70000-70002",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parsePortRange(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePortRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parsePortRange() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid port",
			input:   "8080",
			wantErr: false,
		},
		{
			name:    "minimum valid port",
			input:   "1",
			wantErr: false,
		},
		{
			name:    "maximum valid port",
			input:   "65535",
			wantErr: false,
		},
		{
			name:    "non-numeric",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "port too low",
			input:   "0",
			wantErr: true,
		},
		{
			name:    "port too high",
			input:   "65536",
			wantErr: true,
		},
		{
			name:    "negative port",
			input:   "-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAndDeduplicatePorts(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
		wantErr  bool
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "no duplicates",
			input:    []string{"8080", "8081", "8082"},
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:     "with duplicates",
			input:    []string{"8080", "8081", "8080", "8082"},
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:     "unsorted input",
			input:    []string{"8082", "8080", "8081"},
			expected: []string{"8080", "8081", "8082"},
			wantErr:  false,
		},
		{
			name:     "mixed order with duplicates",
			input:    []string{"8085", "8080", "8082", "8080", "8081"},
			expected: []string{"8080", "8081", "8082", "8085"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateAndDeduplicatePorts(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAndDeduplicatePorts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("validateAndDeduplicatePorts() = %v, want %v", result, tt.expected)
			}
		})
	}
}