package hypervisor

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAutoinstallConfig_Validate(t *testing.T) {
	cases := []struct {
		name string
		c    AutoinstallConfig
		want string
	}{
		{"happy", AutoinstallConfig{
			Hostname: "h", FQDN: "h.example", Username: "u", AuthorizedKey: "ssh-ed25519 AAAA",
		}, ""},
		{"no hostname", AutoinstallConfig{
			FQDN: "f", Username: "u", AuthorizedKey: "k",
		}, "Hostname"},
		{"no fqdn", AutoinstallConfig{
			Hostname: "h", Username: "u", AuthorizedKey: "k",
		}, "FQDN"},
		{"no user", AutoinstallConfig{
			Hostname: "h", FQDN: "f", AuthorizedKey: "k",
		}, "Username"},
		{"no key", AutoinstallConfig{
			Hostname: "h", FQDN: "f", Username: "u",
		}, "AuthorizedKey"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.Validate()
			if tc.want == "" {
				if err != nil {
					t.Errorf("got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestBuildUserData_ShapeMatters(t *testing.T) {
	cfg := &AutoinstallConfig{
		Hostname:      "abcDEFgh",
		FQDN:          "abcDEFgh.example.com",
		Username:      "operator",
		AuthorizedKey: "ssh-ed25519 AAAA fingerprint operator@laptop\n",
	}
	got, err := BuildUserData(cfg)
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)

	for _, want := range []string{
		"#cloud-config",
		"autoinstall:",
		"version: 1",
		"hostname: abcDEFgh",
		"username: operator",
		"install-server: true",
		"allow-pw: false",
		"ssh-ed25519 AAAA fingerprint operator@laptop",
		"openssh-server",
		"layout:\n      name: direct",
		"match:\n        path: /dev/vda",
		"usermod -aG sudo operator",
		"if [ -b /dev/vdb ]",
		"mkfs.ext4 -F /dev/vdb",
		"echo \"/dev/vdb /var ext4 defaults 0 2\"",
		"sudoers.d/90-convocate-vm",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("user-data missing %q\n%s", want, body)
		}
	}

	// AuthorizedKey trailing newline trimmed.
	if strings.Contains(body, "fingerprint operator@laptop\n      -") {
		t.Error("authorized key should be trimmed of trailing newline before injection")
	}
}

func TestBuildUserData_HashedPasswordIncluded(t *testing.T) {
	cfg := &AutoinstallConfig{
		Hostname: "h", FQDN: "h.example", Username: "u",
		AuthorizedKey:  "ssh-ed25519 X",
		HashedPassword: "$6$salt$hash",
	}
	got, _ := BuildUserData(cfg)
	if !strings.Contains(string(got), `password: "$6$salt$hash"`) {
		t.Errorf("hashed password missing:\n%s", got)
	}
}

func TestBuildUserData_NoPasswordWhenEmpty(t *testing.T) {
	cfg := &AutoinstallConfig{
		Hostname: "h", FQDN: "h.example", Username: "u",
		AuthorizedKey: "ssh-ed25519 X",
	}
	got, _ := BuildUserData(cfg)
	if strings.Contains(string(got), "password:") {
		t.Errorf("password field should be absent when empty:\n%s", got)
	}
}

func TestBuildUserData_ValidationFails(t *testing.T) {
	if _, err := BuildUserData(&AutoinstallConfig{}); err == nil {
		t.Error("expected validation error")
	}
}

func TestBuildMetaData(t *testing.T) {
	got, err := BuildMetaData(&AutoinstallConfig{Hostname: "h"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "instance-id: h\nlocal-hostname: h\n" {
		t.Errorf("got %q", got)
	}
}

func TestBuildMetaData_RequiresHostname(t *testing.T) {
	if _, err := BuildMetaData(&AutoinstallConfig{}); err == nil {
		t.Error("expected error")
	}
}

// --- PushAutoinstallSeed --------------------------------------------------

func validCfg() *AutoinstallConfig {
	return &AutoinstallConfig{
		Hostname:      "h",
		FQDN:          "h.example",
		Username:      "u",
		AuthorizedKey: "ssh-ed25519 AAAA u@l",
	}
}

func TestPushAutoinstallSeed_ValidationFails(t *testing.T) {
	m := &mockRunner{}
	if err := PushAutoinstallSeed(context.Background(), m, &AutoinstallConfig{}, "/iso", "seed.iso"); err == nil {
		t.Error("expected validation error")
	}
}

func TestPushAutoinstallSeed_ArgsRequired(t *testing.T) {
	m := &mockRunner{}
	if err := PushAutoinstallSeed(context.Background(), m, validCfg(), "", "seed.iso"); err == nil ||
		!strings.Contains(err.Error(), "isoDir") {
		t.Errorf("expected isoDir error, got %v", err)
	}
	if err := PushAutoinstallSeed(context.Background(), m, validCfg(), "/iso", ""); err == nil ||
		!strings.Contains(err.Error(), "seedName") {
		t.Errorf("expected seedName error, got %v", err)
	}
}

func TestPushAutoinstallSeed_HappyPath(t *testing.T) {
	m := &mockRunner{}
	err := PushAutoinstallSeed(context.Background(), m, validCfg(), "/var/lib/libvirt/images", "seed.iso")
	if err != nil {
		t.Fatalf("PushAutoinstallSeed: %v", err)
	}

	// Two file uploads: user-data + meta-data.
	if len(m.copies) != 2 {
		t.Fatalf("expected 2 copies, got %d", len(m.copies))
	}
	dests := map[string]bool{}
	for _, c := range m.copies {
		dests[c.Dst] = true
		if c.Mode != 0644 {
			t.Errorf("copy %s mode = %o, want 0644", c.Dst, c.Mode)
		}
	}
	for _, want := range []string{
		"/var/lib/libvirt/images/user-data",
		"/var/lib/libvirt/images/meta-data",
	} {
		if !dests[want] {
			t.Errorf("missing copy dest %q", want)
		}
	}

	// Verify content of each file.
	for _, c := range m.copies {
		if strings.HasSuffix(c.Dst, "user-data") {
			if !strings.Contains(string(c.Content), "hostname: h") {
				t.Errorf("user-data content wrong: %s", c.Content)
			}
		}
		if strings.HasSuffix(c.Dst, "meta-data") {
			if !strings.Contains(string(c.Content), "instance-id: h") {
				t.Errorf("meta-data content wrong: %s", c.Content)
			}
		}
	}

	// One mkdir, one seed-build script. Both sudo.
	if len(m.cmds) != 2 {
		t.Fatalf("expected 2 cmds, got %d", len(m.cmds))
	}
	if !strings.Contains(m.cmds[0].Cmd, "mkdir -p '/var/lib/libvirt/images'") {
		t.Errorf("cmd 0 should mkdir, got %q", m.cmds[0].Cmd)
	}
	if !strings.Contains(m.cmds[1].Cmd, "cloud-localds") || !strings.Contains(m.cmds[1].Cmd, "genisoimage") {
		t.Errorf("seed-build script should reference both cloud-localds + genisoimage fallback:\n%s", m.cmds[1].Cmd)
	}
	if !strings.Contains(m.cmds[1].Cmd, "/var/lib/libvirt/images/seed.iso") {
		t.Errorf("seed path missing from script: %s", m.cmds[1].Cmd)
	}
	for _, c := range m.cmds {
		if !c.Opts.Sudo {
			t.Errorf("cmd should run as sudo: %s", c.Cmd)
		}
	}
}

func TestPushAutoinstallSeed_MkdirFails(t *testing.T) {
	m := &mockRunner{failOn: map[string]error{"mkdir -p ": errors.New("perm")}}
	err := PushAutoinstallSeed(context.Background(), m, validCfg(), "/iso", "seed.iso")
	if err == nil || !strings.Contains(err.Error(), "mkdir") {
		t.Errorf("expected mkdir error, got %v", err)
	}
}

func TestPushAutoinstallSeed_BuildSeedFails(t *testing.T) {
	m := &mockRunner{failOn: map[string]error{"set -e\nif command -v cloud-localds": errors.New("disk full")}}
	err := PushAutoinstallSeed(context.Background(), m, validCfg(), "/iso", "seed.iso")
	if err == nil || !strings.Contains(err.Error(), "build seed ISO") {
		t.Errorf("expected build error, got %v", err)
	}
}

func TestPushAutoinstallSeed_UserDataUploadFails(t *testing.T) {
	m := &mockRunner{copyFailOn: map[string]error{"user-data": errors.New("disk full")}}
	err := PushAutoinstallSeed(context.Background(), m, validCfg(), "/iso", "seed.iso")
	if err == nil || !strings.Contains(err.Error(), "upload user-data") {
		t.Errorf("expected user-data upload error, got %v", err)
	}
}

func TestPushAutoinstallSeed_MetaDataUploadFails(t *testing.T) {
	m := &mockRunner{copyFailOn: map[string]error{"meta-data": errors.New("disk full")}}
	err := PushAutoinstallSeed(context.Background(), m, validCfg(), "/iso", "seed.iso")
	if err == nil || !strings.Contains(err.Error(), "upload meta-data") {
		t.Errorf("expected meta-data upload error, got %v", err)
	}
}
