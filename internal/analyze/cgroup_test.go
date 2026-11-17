package analyze

import (
	"context"
	"encoding/json"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"strings"
	"testing"
)

const v1Mount = "31 22 0:27 / /sys/fs/cgroup/memory rw - cgroup cgroup rw,memory\n"
const v2Mount = "32 22 0:28 / /sys/fs/cgroup rw - cgroup2 cgroup rw\n"

func TestCgroupEvidenceClassification(t *testing.T) {
	for _, tc := range []struct {
		s    CgroupSide
		want string
	}{
		{CgroupSide{Scope: "node", MountInfo: v1Mount}, "v1"},
		{CgroupSide{Scope: "node", MountInfo: v2Mount}, "v2"},
		{CgroupSide{Scope: "node", MountInfo: v1Mount + v2Mount}, "hybrid"},
		{CgroupSide{Scope: "node", RootFS: "tmpfs"}, "unknown"},
		{CgroupSide{Scope: "container", RootFS: "cgroup2fs", MountInfo: v2Mount}, "unknown"},
		{CgroupSide{Scope: "node", RootFS: "cgroup2fs"}, "v2"},
		{CgroupSide{Scope: "node", Mode: "v1", Evidence: "old inventory", MountInfo: v2Mount}, "conflict"},
		{CgroupSide{Scope: "node", RootFS: "tmpfs", MountInfo: v2Mount}, "conflict"},
	} {
		got, _, e := cgroupMode(tc.s)
		if e != nil || got != tc.want {
			t.Fatal(got, tc.want, e)
		}
	}
	if _, _, e := cgroupMode(CgroupSide{Scope: "node", MountInfo: "malformed"}); e == nil {
		t.Fatal("malformed evidence accepted")
	}
}
func cgroupFixture() []CgroupPool {
	return []CgroupPool{{Name: "linux", Current: CgroupSide{Scope: "node", MountInfo: v1Mount}, Target: CgroupSide{Mode: "v2", Evidence: "approved target specification", Kernel: "6.8.0-azure", Runtime: "containerd", RuntimeVersion: "2.0.0", KubeletDriver: "systemd", RuntimeDriver: "systemd"}}}
}
func TestCgroupComparisonAndWorkloadEvidence(t *testing.T) {
	set := decode(t, `apiVersion: v1
kind: Pod
metadata: {name: monitor}
spec:
  volumes:
  - name: cgroups
    hostPath: {path: /sys/fs/cgroup}
  containers:
  - name: monitor
    image: example.invalid/monitor:test
    args: ['cat /sys/fs/cgroup/memory/memory.limit_in_bytes; PRIVATE-MARKER']
`)
	p, e := Cgroups(set, cgroupFixture(), "1.36")
	if e != nil || p.Cgroup.Pools[0].Status != "mode_change" {
		t.Fatal(p, e)
	}
	found := map[string]bool{}
	for _, a := range p.Actions {
		found[a.ID] = true
	}
	if !found["KW-CGROUP-V1-REFERENCE"] || !found["KW-CGROUP-HOST-MOUNT"] {
		t.Fatal(p.Actions)
	}
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "PRIVATE-MARKER") || strings.Contains(string(b), v1Mount) {
		t.Fatal("raw input leaked")
	}
	pools := cgroupFixture()
	pools[0].Target.Mode = "v1"
	p, e = Cgroups(set, pools, "1.36")
	if e != nil {
		t.Fatal(e)
	}
	b, _ = json.Marshal(p)
	if !strings.Contains(string(b), "failCgroupV1") {
		t.Fatal("missing 1.35 default warning")
	}
	pools = cgroupFixture()
	pools[0].Target.Kernel = "5.4.0"
	pools[0].Target.RuntimeVersion = "1.3.9"
	pools[0].Target.RuntimeDriver = "cgroupfs"
	p, e = Cgroups(set, pools, "1.36")
	if e != nil {
		t.Fatal(e)
	}
	checks := strings.Join(p.Cgroup.Pools[0].Checks, " ")
	for _, x := range []string{"kernel below", "runtime below", "drivers are incompatible"} {
		if !strings.Contains(checks, x) {
			t.Fatal(checks)
		}
	}
}
func TestCgroupUnknownSameModeAndProjectIntegration(t *testing.T) {
	set := manifest.Set{}
	p, e := Cgroups(set, nil, "1.36")
	if e != nil || len(p.Cgroup.Pools) != 0 || len(p.Actions) != 2 {
		t.Fatal(p, e)
	}
	pools := cgroupFixture()
	pools[0].Current = CgroupSide{Mode: "v2", Evidence: "inventory"}
	p, e = Cgroups(set, pools, "1.36")
	if e != nil || p.Cgroup.Pools[0].Status != "same_mode" {
		t.Fatal(p, e)
	}
	profile := projectFixture()
	profile.CgroupPools = pools
	p, e = Project(context.Background(), set, profile)
	if e != nil || len(p.Cgroup.Pools) != 1 {
		t.Fatal(p, e)
	}
	n := 0
	for _, a := range p.Actions {
		if a.ID == "KW-CGROUP-MODE" {
			n++
		}
	}
	if n != 1 {
		t.Fatal("duplicate comparison actions", n)
	}
	pools[0].Current.Mode = "typo"
	if ValidateCgroups(pools) == nil {
		t.Fatal("invalid mode accepted")
	}
	pools = cgroupFixture()
	pools[0].Target.Evidence = ""
	if ValidateCgroups(pools) == nil {
		t.Fatal("unattributed declaration accepted")
	}
}

func TestCgroupProjectDoesNotEchoMountInfo(t *testing.T) {
	profile := projectFixture()
	profile.CgroupPools = cgroupFixture()
	before := profile.CgroupPools[0].Current.MountInfo
	p, e := Project(context.Background(), manifest.Set{}, profile)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), `"mountinfo"`) {
		t.Fatal("raw mountinfo echoed")
	}
	if profile.CgroupPools[0].Current.MountInfo != before {
		t.Fatal("caller profile mutated")
	}
}
