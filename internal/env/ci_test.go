package env

import (
	"os"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    bool
	}{
		{"CI=true", map[string]string{"CI": "true"}, true},
		{"CI=1", map[string]string{"CI": "1"}, true},
		{"GITHUB_ACTIONS=true", map[string]string{"GITHUB_ACTIONS": "true"}, true},
		{"GITLAB_CI=true", map[string]string{"GITLAB_CI": "true"}, true},
		{"No CI env", map[string]string{"CI": "false"}, false},
		{"Empty env", map[string]string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env
			os.Clearenv()
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			
			// Reset package state
			isCI = false
			
			Init()
			if IsCI() != tt.want {
				t.Errorf("Init() got %v, want %v", IsCI(), tt.want)
			}
		})
	}
}

func TestSetCI(t *testing.T) {
	isCI = false
	SetCI(true)
	if !IsCI() {
		t.Errorf("SetCI(true) failed, got %v", IsCI())
	}
	SetCI(false)
	if IsCI() {
		t.Errorf("SetCI(false) failed, got %v", IsCI())
	}
}
