package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AutoinstallConfig drives the cloud-init NoCloud user-data subiquity
// will pick up at install time. The fields are deliberately minimal —
// just enough to produce an SSH-reachable Ubuntu host the operator can
// then drive with claude-host install / init-agent.
type AutoinstallConfig struct {
	// Hostname is the random 8-char value generated earlier in the flow.
	Hostname string

	// FQDN is "<Hostname>.<Domain>".
	FQDN string

	// Username is the local account created on the new VM. Reuses the
	// operator's username on the hypervisor for parity, but it can be
	// any valid Linux username.
	Username string

	// AuthorizedKey is the operator's pubkey installed into the new
	// account's ~/.ssh/authorized_keys. Without this, the VM is
	// unreachable post-install.
	AuthorizedKey string

	// HashedPassword is the optional SHA-512 crypt for the user
	// (mkpasswd -m sha-512). Empty means key-only auth (recommended).
	HashedPassword string
}

// Validate checks the required fields. Empty Hostname/FQDN/Username/
// AuthorizedKey is a misconfiguration we should fail loudly on rather
// than burying it under a cryptic subiquity error after the VM boots.
func (c *AutoinstallConfig) Validate() error {
	if c.Hostname == "" {
		return errors.New("autoinstall: Hostname required")
	}
	if c.FQDN == "" {
		return errors.New("autoinstall: FQDN required")
	}
	if c.Username == "" {
		return errors.New("autoinstall: Username required")
	}
	if strings.TrimSpace(c.AuthorizedKey) == "" {
		return errors.New("autoinstall: AuthorizedKey required (no SSH key would lock the operator out)")
	}
	return nil
}

// BuildUserData renders the Ubuntu subiquity autoinstall YAML for cfg.
// Storage layout is "direct" on /dev/vda for /; the secondary
// /dev/vdb data disk is formatted + mounted at /var via late-commands
// so this single file covers the dual-disk case the create-vm CLI
// allocates. Without this step /var would land on the OS disk and
// the --datadisk flag would be invisible to the running VM.
//
// SSH server is installed; password auth is disabled at the cloud-init
// stage too so the VM matches the CIS-aligned posture HardenSSHD
// produces on the hypervisor itself.
func BuildUserData(cfg *AutoinstallConfig) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	pwLine := ""
	if cfg.HashedPassword != "" {
		pwLine = fmt.Sprintf("    password: %q\n", cfg.HashedPassword)
	}
	body := fmt.Sprintf(`#cloud-config
autoinstall:
  version: 1
  refresh-installer:
    update: yes
  apt:
    preserve_sources_list: true
  identity:
    realname: claude-shell VM
    hostname: %s
    username: %s
%s  ssh:
    install-server: true
    allow-pw: false
    authorized-keys:
      - %s
  packages:
    - openssh-server
    - sudo
    - cloud-init
    - python3
  storage:
    swap:
      size: 0
    grub:
      reorder_uefi: false
    layout:
      name: direct
      match:
        path: /dev/vda
  late-commands:
    - curtin in-target --target=/target -- usermod -aG sudo %s
    - |
      if [ -b /dev/vdb ]; then
        mkfs.ext4 -F /dev/vdb
        mkdir -p /target/var
        echo "/dev/vdb /var ext4 defaults 0 2" >> /target/etc/fstab
      fi
    - echo '%s ALL=(ALL) NOPASSWD:ALL' > /target/etc/sudoers.d/90-claude-vm
    - chmod 0440 /target/etc/sudoers.d/90-claude-vm
`,
		cfg.Hostname,
		cfg.Username,
		pwLine,
		strings.TrimSpace(cfg.AuthorizedKey),
		cfg.Username,
		cfg.Username,
	)
	return []byte(body), nil
}

// BuildMetaData renders the NoCloud meta-data file. Just enough to
// satisfy cloud-init's datasource — instance-id is required;
// local-hostname matches what subiquity will set so cloud-init's
// post-boot rename is a no-op.
func BuildMetaData(cfg *AutoinstallConfig) ([]byte, error) {
	if cfg.Hostname == "" {
		return nil, errors.New("BuildMetaData: Hostname required")
	}
	body := fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", cfg.Hostname, cfg.Hostname)
	return []byte(body), nil
}

// PushAutoinstallSeed lays user-data + meta-data into a temp dir on
// the hypervisor and packages them into a NoCloud seed ISO using
// cloud-localds (preferred) or genisoimage as a fallback. The seed
// ISO ends up at <isoDir>/<seedName>; the orchestrator passes it to
// virt-install as a second --cdrom alongside the Ubuntu ISO.
func PushAutoinstallSeed(ctx context.Context, r Runner, cfg *AutoinstallConfig, isoDir, seedName string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if isoDir == "" {
		return errors.New("PushAutoinstallSeed: isoDir required")
	}
	if seedName == "" {
		return errors.New("PushAutoinstallSeed: seedName required")
	}

	userData, err := BuildUserData(cfg)
	if err != nil {
		return err
	}
	metaData, err := BuildMetaData(cfg)
	if err != nil {
		return err
	}

	// Stage the two files locally then upload via CopyFile so the
	// content is identical regardless of operator shell quoting.
	tmp, err := os.MkdirTemp("", "claude-seed-*")
	if err != nil {
		return fmt.Errorf("local mktemp: %w", err)
	}
	defer os.RemoveAll(tmp)
	udLocal := filepath.Join(tmp, "user-data")
	mdLocal := filepath.Join(tmp, "meta-data")
	if err := os.WriteFile(udLocal, userData, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(mdLocal, metaData, 0644); err != nil {
		return err
	}

	udRemote := filepath.Join(isoDir, "user-data")
	mdRemote := filepath.Join(isoDir, "meta-data")
	seedRemote := filepath.Join(isoDir, seedName)

	if err := r.Run(ctx, "mkdir -p "+shellQuoteArg(isoDir), RunOptions{Sudo: true}); err != nil {
		return fmt.Errorf("mkdir %s: %w", isoDir, err)
	}
	if err := r.CopyFile(ctx, udLocal, udRemote, 0644); err != nil {
		return fmt.Errorf("upload user-data: %w", err)
	}
	if err := r.CopyFile(ctx, mdLocal, mdRemote, 0644); err != nil {
		return fmt.Errorf("upload meta-data: %w", err)
	}

	// cloud-image-utils ships cloud-localds; if it's not present we
	// fall back to genisoimage which is more universally available.
	// Both produce a NoCloud-shaped ISO (ISO 9660 + Joliet/Rock,
	// volume label CIDATA).
	cmd := fmt.Sprintf(`set -e
if command -v cloud-localds >/dev/null 2>&1; then
  cloud-localds %[1]s %[2]s %[3]s
else
  apt-get install -y genisoimage cloud-image-utils >/dev/null 2>&1 || true
  if command -v cloud-localds >/dev/null 2>&1; then
    cloud-localds %[1]s %[2]s %[3]s
  else
    genisoimage -output %[1]s -volid CIDATA -joliet -rock %[2]s %[3]s
  fi
fi
`, shellQuoteArg(seedRemote), shellQuoteArg(udRemote), shellQuoteArg(mdRemote))

	if err := r.Run(ctx, cmd, RunOptions{Sudo: true}); err != nil {
		return fmt.Errorf("build seed ISO: %w", err)
	}
	return nil
}
