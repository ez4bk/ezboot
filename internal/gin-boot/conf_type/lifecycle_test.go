package conf_type

import (
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

func TestLifecycleYAML(t *testing.T) {
	var cfg struct {
		Lifecycle Lifecycle `yaml:"lifecycle"`
	}

	raw := "lifecycle:\n" +
		"  ready_timeout: 10s\n" +
		"  drain_timeout: 2m\n" +
		"  close_timeout: 500ms\n"

	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := cfg.Lifecycle.ReadyTimeout.Duration; got != 10*time.Second {
		t.Fatalf("ready_timeout = %v", got)
	}
	if got := cfg.Lifecycle.DrainTimeout.Duration; got != 2*time.Minute {
		t.Fatalf("drain_timeout = %v", got)
	}
	if got := cfg.Lifecycle.CloseTimeout.Duration; got != 500*time.Millisecond {
		t.Fatalf("close_timeout = %v", got)
	}
}
