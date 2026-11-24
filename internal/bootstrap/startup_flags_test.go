package bootstrap

import (
	"os"
	"testing"
)

func TestParseStartupOptions(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantMigrate *bool
		wantSeed    *bool
		wantErr     bool
	}{
		{name: "bare start sets nothing", args: nil},
		{name: "migrate flag", args: []string{"-migrate"}, wantMigrate: boolPtr(true)},
		{name: "migrate explicitly disabled", args: []string{"-migrate=false"}, wantMigrate: boolPtr(false)},
		{name: "seed flag", args: []string{"-seed"}, wantSeed: boolPtr(true)},
		{name: "both flags", args: []string{"--migrate", "--seed"}, wantMigrate: boolPtr(true), wantSeed: boolPtr(true)},
		{name: "positional argument is rejected", args: []string{"up"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := ParseStartupOptions(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertBoolPtr(t, "migrate", opts.Migrate, tc.wantMigrate)
			assertBoolPtr(t, "seed", opts.Seed, tc.wantSeed)
		})
	}
}

func TestResolveStartupGatesPrecedence(t *testing.T) {
	env := func(values map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) {
			v, ok := values[key]
			return v, ok
		}
	}

	tests := []struct {
		name           string
		opts           StartupOptions
		env            map[string]string
		wantMigrate    bool
		wantSeed       bool
		wantMigrateSrc string
	}{
		{
			name: "defaults are off",
			env:  map[string]string{},
		},
		{
			name:        "env enables migrate",
			env:         map[string]string{"AUTO_MIGRATE": "true"},
			wantMigrate: true, wantMigrateSrc: "env",
		},
		{
			name:        "env enables both",
			env:         map[string]string{"AUTO_MIGRATE": "1", "AUTO_SEED": "yes"},
			wantMigrate: true, wantSeed: true, wantMigrateSrc: "env",
		},
		{
			name:           "flag wins over env",
			opts:           StartupOptions{Migrate: boolPtr(false), Seed: boolPtr(false)},
			env:            map[string]string{"AUTO_MIGRATE": "true", "AUTO_SEED": "true"},
			wantMigrateSrc: "flag",
		},
		{
			name:        "seed implies migrate",
			opts:        StartupOptions{Seed: boolPtr(true)},
			env:         map[string]string{"AUTO_MIGRATE": "false"},
			wantMigrate: true, wantSeed: true, wantMigrateSrc: "implied-by-seed",
		},
		{
			name:        "env seed implies migrate",
			env:         map[string]string{"AUTO_SEED": "on"},
			wantMigrate: true, wantSeed: true, wantMigrateSrc: "implied-by-seed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := resolveStartupGates(tc.opts, env(tc.env))
			if d.migrate != tc.wantMigrate || d.seed != tc.wantSeed {
				t.Fatalf("gates = (migrate=%v seed=%v), want (%v, %v)", d.migrate, d.seed, tc.wantMigrate, tc.wantSeed)
			}
			if tc.wantMigrateSrc != "" && d.migrateSource != tc.wantMigrateSrc {
				t.Errorf("migrateSource = %q, want %q", d.migrateSource, tc.wantMigrateSrc)
			}
		})
	}
}

// TestStartupGateDecisionApplyWritesEnv 固定「裁决写回环境变量」这一事实：
// db.Migrate 与凭据初始化都从环境读取，两层必须看到同一结论。
func TestStartupGateDecisionApplyWritesEnv(t *testing.T) {
	t.Setenv("AUTO_MIGRATE", "false")
	t.Setenv("AUTO_SEED", "false")

	decision := startupGateDecision{migrate: true, seed: false}
	decision.apply()

	if got := os.Getenv("AUTO_MIGRATE"); got != "true" {
		t.Fatalf("AUTO_MIGRATE = %q, want true", got)
	}
	if got := os.Getenv("AUTO_SEED"); got != "false" {
		t.Fatalf("AUTO_SEED = %q, want false", got)
	}
}

func TestParseStartupOptionsHelp(t *testing.T) {
	_, err := ParseStartupOptions([]string{"-h"})
	if err == nil {
		t.Fatal("expected flag.ErrHelp")
	}
}

func assertBoolPtr(t *testing.T, name string, got, want *bool) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("%s = %v, want <unset>", name, *got)
		}
		return
	}
	if got == nil {
		t.Fatalf("%s = <unset>, want %v", name, *want)
	}
	if *got != *want {
		t.Fatalf("%s = %v, want %v", name, *got, *want)
	}
}

func boolPtr(v bool) *bool { return &v }
