package hypervisor

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// --- listDomainNames -------------------------------------------------------

func TestListDomainNames_Empty(t *testing.T) {
	m := &mockRunner{cmdStdout: map[string]string{"virsh list --all --name": "\n"}}
	got, err := listDomainNames(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestListDomainNames_TwoVMs(t *testing.T) {
	m := &mockRunner{cmdStdout: map[string]string{"virsh list --all --name": "vm-a\nvm-b\n"}}
	got, err := listDomainNames(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"vm-a", "vm-b"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestListDomainNames_RunFails(t *testing.T) {
	m := &mockRunner{failOn: map[string]error{"virsh list --all --name": errors.New("libvirt down")}}
	if _, err := listDomainNames(context.Background(), m); err == nil {
		t.Error("expected propagated error")
	}
}

// --- readDomainInfo --------------------------------------------------------

func TestReadDomainInfo_Parses(t *testing.T) {
	m := &mockRunner{cmdStdout: map[string]string{
		"virsh dominfo": `Id:             3
Name:           vm-x
UUID:           00000000-0000-0000-0000-000000000001
OS Type:        hvm
State:          running
CPU(s):         4
Max memory:     8388608 KiB
Used memory:    2097152 KiB
`,
	}}
	info, err := readDomainInfo(context.Background(), m, "vm-x")
	if err != nil {
		t.Fatal(err)
	}
	if info.CPUCores != 4 {
		t.Errorf("CPUCores = %d", info.CPUCores)
	}
	if info.RAMMB != 8192 {
		t.Errorf("RAMMB = %d, want 8192", info.RAMMB)
	}
}

func TestReadDomainInfo_RunFails(t *testing.T) {
	m := &mockRunner{failOn: map[string]error{"virsh dominfo": errors.New("nope")}}
	if _, err := readDomainInfo(context.Background(), m, "x"); err == nil {
		t.Error("expected error")
	}
}

func TestReadDomainInfo_PartialOutput(t *testing.T) {
	// Output without CPU(s) or Max memory lines — info should be zero,
	// no error.
	m := &mockRunner{cmdStdout: map[string]string{
		"virsh dominfo": "Id:  -\nName: x\n",
	}}
	info, _ := readDomainInfo(context.Background(), m, "x")
	if info.CPUCores != 0 || info.RAMMB != 0 {
		t.Errorf("expected zeros, got %+v", info)
	}
}

// --- totalPoolCapacityGB ---------------------------------------------------

func TestTotalPoolCapacityGB_Sums(t *testing.T) {
	m := &mockRunner{cmdStdout: map[string]string{
		"virsh vol-list --pool default --details": ` Name             Path                          Type    Capacity     Allocation
-----------------------------------------------------------------------------------------
 a.qcow2          /var/lib/libvirt/images/a     file    50.00 GiB    32.00 GiB
 b.qcow2          /var/lib/libvirt/images/b     file    1.00 TiB     0.50 TiB
 c.qcow2          /var/lib/libvirt/images/c     file    512 MiB      256 MiB
`,
	}}
	got, err := totalPoolCapacityGB(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	// 50 + 1024 + 0 (512 MiB rounds to 0 GiB by truncation of 0.5)
	if got != 1074 {
		t.Errorf("got %d, want 1074", got)
	}
}

func TestTotalPoolCapacityGB_EmptyPool(t *testing.T) {
	m := &mockRunner{cmdStdout: map[string]string{
		"virsh vol-list --pool default --details": ` Name  Path  Type  Capacity  Allocation
---
`,
	}}
	got, err := totalPoolCapacityGB(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestTotalPoolCapacityGB_RunFails(t *testing.T) {
	m := &mockRunner{failOn: map[string]error{"virsh vol-list": errors.New("no pool")}}
	if _, err := totalPoolCapacityGB(context.Background(), m); err == nil {
		t.Error("expected error")
	}
}

func TestParseGiB(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"50.00 GiB", 50, true},
		{"1.50 TiB", 1536, true},
		{"100 MiB", 0, true}, // rounds to 0 GiB
		{"42", 0, false},
		{"", 0, false},
		{"50 KB", 0, false},
		{"abc GiB", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseGiB(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseGiB(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// --- QueryExistingPledge full flow -----------------------------------------

func TestQueryExistingPledge_HappyPath(t *testing.T) {
	m := &mockRunner{cmdStdout: map[string]string{
		"virsh list --all --name": "vm-a\nvm-b\n",
		"virsh dominfo 'vm-a'": `CPU(s):         2
Max memory:     2097152 KiB
`,
		"virsh dominfo 'vm-b'": `CPU(s):         4
Max memory:     8388608 KiB
`,
		"virsh vol-list --pool default --details": `Name Path Type Capacity Allocation
---
 disk-a /p file  20.00 GiB  10.00 GiB
 disk-b /p file  100.00 GiB 50.00 GiB
`,
	}}
	p, err := QueryExistingPledge(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if p.CPUCores != 6 {
		t.Errorf("CPU = %d, want 6", p.CPUCores)
	}
	if p.RAMMB != 10240 {
		t.Errorf("RAM = %d, want 10240", p.RAMMB)
	}
	if p.DiskGB != 120 {
		t.Errorf("Disk = %d, want 120", p.DiskGB)
	}
}

func TestQueryExistingPledge_DominfoFailure_PartialResult(t *testing.T) {
	// One VM's dominfo errors — total should still account for the
	// good VM rather than failing the whole admission check.
	m := &mockRunner{
		cmdStdout: map[string]string{
			"virsh list --all --name": "good\nbroken\n",
			"virsh dominfo 'good'": `CPU(s):         3
Max memory:     1048576 KiB
`,
		},
		failOn: map[string]error{"virsh dominfo 'broken'": errors.New("not found")},
	}
	p, err := QueryExistingPledge(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if p.CPUCores != 3 {
		t.Errorf("expected partial sum, got %+v", p)
	}
}

func TestQueryExistingPledge_ListFails(t *testing.T) {
	m := &mockRunner{failOn: map[string]error{"virsh list --all --name": errors.New("nope")}}
	if _, err := QueryExistingPledge(context.Background(), m); err == nil ||
		!strings.Contains(err.Error(), "list domains") {
		t.Errorf("expected list error, got %v", err)
	}
}

func TestQueryExistingPledge_VolListFailure_NoDisk(t *testing.T) {
	// vol-list errors but the function continues and just returns 0
	// for disk — the CPU/RAM sums are still useful.
	m := &mockRunner{
		cmdStdout: map[string]string{"virsh list --all --name": "x\n"},
		failOn: map[string]error{
			"virsh dominfo 'x'": errors.New("partial"),
			"virsh vol-list":    errors.New("no pool yet"),
		},
	}
	p, err := QueryExistingPledge(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if p.DiskGB != 0 {
		t.Errorf("disk should be 0 when vol-list errors, got %d", p.DiskGB)
	}
}

// --- CheckAdmission --------------------------------------------------------

func TestCheckAdmission_AllAxesFit(t *testing.T) {
	host := HypervisorResources{CPUCores: 16, RAMMB: 32768, DiskGB: 1000}
	existing := Pledge{CPUCores: 4, RAMMB: 8192, DiskGB: 200}
	want := Pledge{CPUCores: 4, RAMMB: 8192, DiskGB: 200}
	if err := CheckAdmission(host, existing, want); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestCheckAdmission_CPUExceeds(t *testing.T) {
	host := HypervisorResources{CPUCores: 4, RAMMB: 8192, DiskGB: 100}
	// 90% of 4 cores = 3.6 → cap = 3 (int truncation: 4*90/100 = 3).
	// existing 2 + want 2 = 4 > 3 → fail.
	err := CheckAdmission(host, Pledge{CPUCores: 2}, Pledge{CPUCores: 2})
	if err == nil || !strings.Contains(err.Error(), "CPU cores") {
		t.Errorf("expected CPU error, got %v", err)
	}
	// Error message is quantitative for operator visibility.
	if !strings.Contains(err.Error(), "would exceed 90%") {
		t.Errorf("error should mention 90%% cap, got %v", err)
	}
}

func TestCheckAdmission_RAMExceeds(t *testing.T) {
	host := HypervisorResources{CPUCores: 100, RAMMB: 1000, DiskGB: 1000}
	err := CheckAdmission(host, Pledge{RAMMB: 800}, Pledge{RAMMB: 200})
	if err == nil || !strings.Contains(err.Error(), "RAM") {
		t.Errorf("expected RAM error, got %v", err)
	}
}

func TestCheckAdmission_DiskExceeds(t *testing.T) {
	host := HypervisorResources{CPUCores: 100, RAMMB: 100000, DiskGB: 100}
	err := CheckAdmission(host, Pledge{DiskGB: 80}, Pledge{DiskGB: 20})
	if err == nil || !strings.Contains(err.Error(), "disk") {
		t.Errorf("expected disk error, got %v", err)
	}
}

func TestCheckAdmission_DiskAxisSkippedWhenHostUnknown(t *testing.T) {
	// host.DiskGB == 0 → DetectResources couldn't measure — admission
	// should pass on disk axis even if the request is large, rather
	// than accidentally always-failing.
	host := HypervisorResources{CPUCores: 100, RAMMB: 100000, DiskGB: 0}
	if err := CheckAdmission(host, Pledge{}, Pledge{DiskGB: 999999}); err != nil {
		t.Errorf("expected admission pass when host disk unknown, got %v", err)
	}
}
