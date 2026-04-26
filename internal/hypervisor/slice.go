package hypervisor

import (
	"context"
	"fmt"
)

// CapMachineSlice writes a systemd drop-in that caps libvirt's
// machine.slice (under which every VM systemd-machined manages runs)
// at 90% of the host's CPU and memory. Pairs with the Layer 1
// admission control: Layer 1 refuses the create call, Layer 2 keeps
// the kernel honest if VMs were ever created out-of-band.
//
// Drop-in path: /etc/systemd/system/machine.slice.d/99-convocate-cap.conf
// — the 99- prefix wins against any other override systemd-machined
// or virt-install might lay down.
//
// CPUQuota is "<nproc * 90>%" so 4 cores cap at 360%, 16 cores at
// 1440%. systemd reads CPUQuota in units of one full core (100% per
// core), so this is the canonical way to express "90% of the box".
//
// MemoryMax is set in absolute bytes from /proc/meminfo MemTotal *
// 0.9. Below that floor and the kernel OOMs the offender; above and
// the slice is over-provisioned.
func CapMachineSlice(ctx context.Context, r Runner, res HypervisorResources) error {
	cpuPct := res.CPUCores * 90
	memBytes := res.RAMMB * 1024 * 1024 * 90 / 100

	unit := fmt.Sprintf(`[Unit]
Description=Cap libvirt VMs at 90%% of host resources (managed by convocate-host create-vm)

[Slice]
CPUAccounting=yes
CPUQuota=%d%%
MemoryAccounting=yes
MemoryMax=%d
`, cpuPct, memBytes)

	cmd := fmt.Sprintf(`set -e
mkdir -p /etc/systemd/system/machine.slice.d
cat >/etc/systemd/system/machine.slice.d/99-convocate-cap.conf <<'CAP_EOF'
%sCAP_EOF
systemctl daemon-reload
# Slice can't be "started" if no VM is in it yet; ignore the error.
systemctl restart machine.slice 2>/dev/null || true
`, unit)

	return r.Run(ctx, cmd, RunOptions{Sudo: true, Stderr: nil})
}
