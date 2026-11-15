package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScan(t *testing.T) {
	cases := []struct{ name, body, target, rule string }{
		{"removed", "apiVersion: batch/v1beta1\nkind: CronJob\n", "1.25", "KW-API-REMOVED"},
		{"before removal", "apiVersion: batch/v1beta1\nkind: CronJob\n", "1.24", "KW-API-DEPRECATED"},
		{"duplicate", "apiVersion: v1\nkind: Secret\nkind: Pod\n", "1.32", "KW-INPUT"},
		{"syntax", "apiVersion: [\n", "1.32", "KW-INPUT"},
		{"type", "apiVersion: 12\nkind: Pod\n", "1.32", "KW-REQUIRED"},
		{"custom", "apiVersion: example.org/v1\nkind: Custom\n", "1.32", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "a.yaml")
			os.WriteFile(p, []byte(c.body), 0600)
			r, e := Scan(p, c.target)
			if e != nil {
				t.Fatal(e)
			}
			if r.Status != "partial" || len(r.Unchecked) == 0 {
				t.Fatal("missing coverage")
			}
			if c.rule == "" {
				if len(r.Findings) != 0 {
					t.Fatal(r)
				}
			} else if len(r.Findings) == 0 || r.Findings[0].Rule != c.rule {
				t.Fatal(r)
			}
		})
	}
}
func TestPrivacyAndMultiDocument(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.yaml")
	os.WriteFile(p, []byte("apiVersion: v1\nkind: Secret\nstringData:\n  password: TOPSECRET\n---\napiVersion: batch/v1beta1\nkind: CronJob\n"), 0600)
	r, e := Scan(p, "1.25")
	if e != nil || r.Documents != 2 || len(r.Findings) != 1 {
		t.Fatal(r, e)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "TOPSECRET") {
		t.Fatal("secret leaked")
	}
}
func TestInvalidInput(t *testing.T) {
	for _, v := range []string{"", "1.99", "1.25junk", "2.1"} {
		if _, e := Target(v); e == nil {
			t.Fatal(v)
		}
	}
	if _, e := Scan(t.TempDir(), "1.32"); e == nil {
		t.Fatal("empty directory accepted")
	}
}
