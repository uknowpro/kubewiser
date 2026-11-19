package cli

import (
	"strings"
	"testing"
)

func TestMatrixMergedGroupsAndPools(t *testing.T) {
	rows := []any{}
	for _, pool := range []string{"pool-b", "pool-a"} {
		for _, topic := range []string{"cgroup", "kernel", "runtime", "drivers"} {
			rows = append(rows, map[string]any{"topic": topic, "resource": pool, "current": "v1", "target": "v2", "status": "input_missing"})
		}
	}
	rows = append(rows, map[string]any{"topic": "api", "resource": "<script>alert(1)</script>", "current": "<img src=x>", "target": "1.36", "status": "action_required"})
	root := map[string]any{"matrix": rows}
	var b strings.Builder
	matrixHTML(&b, root, "ko")
	s := b.String()
	if strings.Count(s, `rowspan="4"`) != 2 || !strings.Contains(s, `class="matrix-group" rowspan="8"`) {
		t.Fatal("incorrect spans", s)
	}
	if strings.Count(s, "pool-a") != 1 || strings.Count(s, "pool-b") != 1 {
		t.Fatal("pool repeated")
	}
	if strings.Index(s, "pool-a") > strings.Index(s, "pool-b") {
		t.Fatal("unstable grouping")
	}
	if strings.Contains(s, "<script>") || strings.Contains(s, "<img src=x>") {
		t.Fatal("unescaped content")
	}
	if strings.Count(s, "<td>입력 부족</td>") != 8 {
		t.Fatal("verdicts must not be merged")
	}
	b.Reset()
	matrixMarkdown(&b, root, "ko")
	if !strings.Contains(b.String(), "### 플랫폼 · 노드") {
		t.Fatal(b.String())
	}
}
