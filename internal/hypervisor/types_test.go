package hypervisor

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCreateVMOptions_Validate(t *testing.T) {
	cases := []struct {
		name string
		opts CreateVMOptions
		want string // substring expected in error; "" means no error
	}{
		{"happy path", CreateVMOptions{
			Hypervisor: "h", Username: "u", Domain: "d",
			CPU: 2, RAMMB: 2048, OSDiskGB: 10, DataDiskGB: 10,
		}, ""},
		{"missing hypervisor", CreateVMOptions{
			Username: "u", Domain: "d",
			CPU: 2, RAMMB: 2048, OSDiskGB: 10, DataDiskGB: 10,
		}, "hypervisor"},
		{"missing username", CreateVMOptions{
			Hypervisor: "h", Domain: "d",
			CPU: 2, RAMMB: 2048, OSDiskGB: 10, DataDiskGB: 10,
		}, "username"},
		{"missing domain", CreateVMOptions{
			Hypervisor: "h", Username: "u",
			CPU: 2, RAMMB: 2048, OSDiskGB: 10, DataDiskGB: 10,
		}, "domain"},
		{"cpu zero", CreateVMOptions{
			Hypervisor: "h", Username: "u", Domain: "d",
			RAMMB: 2048, OSDiskGB: 10, DataDiskGB: 10,
		}, "cpu"},
		{"ram too low", CreateVMOptions{
			Hypervisor: "h", Username: "u", Domain: "d",
			CPU: 2, RAMMB: 256, OSDiskGB: 10, DataDiskGB: 10,
		}, "ram"},
		{"osdisk too small", CreateVMOptions{
			Hypervisor: "h", Username: "u", Domain: "d",
			CPU: 2, RAMMB: 2048, OSDiskGB: 1, DataDiskGB: 10,
		}, "osdisk"},
		{"datadisk zero", CreateVMOptions{
			Hypervisor: "h", Username: "u", Domain: "d",
			CPU: 2, RAMMB: 2048, OSDiskGB: 10,
		}, "datadisk"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.Validate()
			if tc.want == "" {
				if err != nil {
					t.Errorf("expected nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Errorf("error %q missing %q", err, tc.want)
			}
		})
	}
}

func TestCreateVMOptions_WithDefaults(t *testing.T) {
	o := &CreateVMOptions{}
	d := o.withDefaults()
	if d.Stdin == nil {
		t.Error("Stdin default not applied")
	}
	if d.Stderr == nil {
		t.Error("Stderr default not applied")
	}
	if o == d {
		t.Error("withDefaults should return a copy, not mutate original")
	}
}

func TestCreateVM_ValidationFails(t *testing.T) {
	// Validation runs before requireShellHost / runCreateVM, so this
	// errors regardless of host state.
	err := CreateVM(context.Background(), &CreateVMOptions{})
	if err == nil || !strings.Contains(err.Error(), "hypervisor") {
		t.Errorf("expected hypervisor-required, got %v", err)
	}
}

func TestCreateVM_ShellHostCheckRuns(t *testing.T) {
	// Stub requireShellHost + runCreateVM so we can verify the order:
	// validation → shell-host check → orchestrator.
	origCheck := requireShellHost
	origRun := runCreateVM
	defer func() {
		requireShellHost = origCheck
		runCreateVM = origRun
	}()

	called := []string{}
	requireShellHost = func() error {
		called = append(called, "check")
		return nil
	}
	runCreateVM = func(ctx context.Context, opts *CreateVMOptions) error {
		called = append(called, "run")
		return nil
	}
	opts := &CreateVMOptions{
		Hypervisor: "h", Username: "u", Domain: "d",
		CPU: 1, RAMMB: 512, OSDiskGB: 5, DataDiskGB: 1,
	}
	if err := CreateVM(context.Background(), opts); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(called) != 2 || called[0] != "check" || called[1] != "run" {
		t.Errorf("call order = %v, want [check run]", called)
	}
}

func TestCreateVM_ShellHostCheckFailureShortCircuits(t *testing.T) {
	origCheck := requireShellHost
	origRun := runCreateVM
	defer func() {
		requireShellHost = origCheck
		runCreateVM = origRun
	}()

	requireShellHost = func() error { return errors.New("not a shell host") }
	runCreateVM = func(ctx context.Context, opts *CreateVMOptions) error {
		t.Error("orchestrator should not run when shell-host check fails")
		return nil
	}
	opts := &CreateVMOptions{
		Hypervisor: "h", Username: "u", Domain: "d",
		CPU: 1, RAMMB: 512, OSDiskGB: 5, DataDiskGB: 1,
	}
	err := CreateVM(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "not a shell host") {
		t.Errorf("expected shell-host error, got %v", err)
	}
}

func TestShellHostCheck_RealHostMissing(t *testing.T) {
	// On a runner without /usr/local/bin/convocate installed, the
	// real check returns an error. We can't reliably verify it
	// returns nil because some CI environments do install that
	// binary, so we just assert it returns *some* error path or no
	// error path without panicking.
	if err := shellHostCheck(); err != nil {
		// Confirm the message names a missing path so the operator
		// learns what to do next.
		if !strings.Contains(err.Error(), "convocate") &&
			!strings.Contains(err.Error(), "convocate install") {
			t.Errorf("error message should reference convocate: %v", err)
		}
	}
}
