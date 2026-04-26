// Package hypervisor implements `convocate-host create-vm`: provisions a
// vanilla Ubuntu host into a KVM hypervisor (SSH hardening, dnsmasq
// peering, KVM stack install, machine.slice cgroup cap, capacity
// admission), then bootstraps a fully-unattended Ubuntu VM under it
// using cloud-init NoCloud autoinstall.
//
// The package is split into single-responsibility files (runner, iso,
// autoinstall, kvm, capacity, slice, hostname, dnsmasq, create) so each
// piece is unit-testable in isolation. Every external command goes
// through a Runner interface so tests can substitute a recording mock
// without invoking real ssh / virsh / virt-install / docker.
package hypervisor

import (
	"context"
	"errors"
	"io"
	"os"
)

// CreateVMOptions configures a `convocate-host create-vm` invocation.
// Required fields are documented inline; unset required fields surface
// as a clear validation error from CreateVM, before any remote work
// runs.
type CreateVMOptions struct {
	// Hypervisor is the FQDN or IP of the target machine. The host
	// must be a vanilla Ubuntu 22.04+ install reachable on tcp/22.
	Hypervisor string

	// Username is the operator's account on the hypervisor. Initial
	// connection tries key auth first; if that fails the prompt
	// callback is consulted. After our key is installed, subsequent
	// invocations should reach the host without a prompt.
	Username string

	// Domain is the suffix appended to the random 8-char hostname when
	// registering the new host in the shell's dnsmasq. Required —
	// hostnames without a domain don't resolve cluster-wide.
	Domain string

	// CPU is the number of vCPUs the new VM will request.
	CPU int

	// RAM is the new VM's memory in MB.
	RAMMB int

	// OSDiskGB is the size of the root virtual disk for the VM.
	OSDiskGB int

	// DataDiskGB is the size of the secondary "/var" virtual disk.
	DataDiskGB int

	// IsoCacheDir overrides where the Ubuntu ISO is cached locally.
	// Empty defaults to ~/.convocate/iso/.
	IsoCacheDir string

	// HypervisorISODir overrides where the ISO is staged on the
	// hypervisor before virt-install consumes it. Empty defaults to
	// /var/lib/libvirt/images/iso/.
	HypervisorISODir string

	// SeedISOName is the filename used for the cloud-init NoCloud
	// seed ISO on the hypervisor. Empty defaults to
	// "convocate-seed-<hostname>.iso".
	SeedISOName string

	// Stdin is consulted by the password fallback prompt. Defaults to
	// os.Stdin when nil.
	Stdin io.Reader

	// Stderr receives prompts + progress lines. Defaults to os.Stderr.
	Stderr io.Writer

	// PromptPassword overrides the password-prompt logic. Tests
	// substitute a deterministic returner; production reads via
	// term.ReadPassword from the controlling tty.
	PromptPassword func(user, host string) (string, error)
}

// Validate checks the required fields and returns a clear error if any
// are missing or out of range. Called before any remote work begins so
// failure surfaces immediately to the operator.
func (o *CreateVMOptions) Validate() error {
	if o.Hypervisor == "" {
		return errors.New("create-vm: --hypervisor is required")
	}
	if o.Username == "" {
		return errors.New("create-vm: --username is required")
	}
	if o.Domain == "" {
		return errors.New("create-vm: --domain is required")
	}
	if o.CPU < 1 {
		return errors.New("create-vm: --cpu must be >= 1")
	}
	if o.RAMMB < 512 {
		return errors.New("create-vm: --ram must be >= 512 (MB)")
	}
	if o.OSDiskGB < 5 {
		return errors.New("create-vm: --osdisk must be >= 5 (GB)")
	}
	if o.DataDiskGB < 1 {
		return errors.New("create-vm: --datadisk must be >= 1 (GB)")
	}
	return nil
}

// withDefaults returns a copy of opts with fallback streams + paths
// filled in. Returned by reference so the orchestrator can pass it
// through unchanged.
func (o *CreateVMOptions) withDefaults() *CreateVMOptions {
	cp := *o
	if cp.Stdin == nil {
		cp.Stdin = os.Stdin
	}
	if cp.Stderr == nil {
		cp.Stderr = os.Stderr
	}
	return &cp
}

// CreateVM is the top-level entry point invoked by cmd/convocate-host. It
// runs the full provisioning flow against the host described in opts.
// Implemented in create.go once every supporting subsystem is in place.
func CreateVM(ctx context.Context, opts *CreateVMOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	opts = opts.withDefaults()
	if err := requireShellHost(); err != nil {
		return err
	}
	return runCreateVM(ctx, opts)
}

// requireShellHost refuses to run create-vm unless this machine looks
// like a configured convocate host: the binary is installed and the
// dnsmasq integration directory exists so we can register the new
// hypervisor's A record. Any other host running create-vm would silently
// fail to publish DNS, which is exactly the kind of half-success the
// CLAUDE.md project guidance tells us to refuse upfront.
//
// Overridable for tests via shellHostCheck below.
var requireShellHost = func() error { return shellHostCheck() }

func shellHostCheck() error {
	for _, p := range []string{
		"/usr/local/bin/convocate",
		"/var/lib/convocate",
	} {
		if _, err := os.Stat(p); err != nil {
			return errors.New("create-vm: this host is not a configured convocate host (missing " + p + "); run convocate-host install + convocate install first")
		}
	}
	return nil
}

// runCreateVM is the orchestrator hook implemented in create.go. Pulled
// out as a package-level var so tests can stub the whole flow when
// they only care about CLI surface or option validation.
var runCreateVM = func(ctx context.Context, opts *CreateVMOptions) error {
	return errors.New("create-vm: orchestrator not yet wired (see create.go)")
}
