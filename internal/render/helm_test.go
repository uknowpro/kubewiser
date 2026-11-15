package render

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineHelm(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "templates"), 0700)
	os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\nname: test\nversion: 1.0.0\n"), 0600)
	os.WriteFile(filepath.Join(dir, "values.schema.json"), []byte(`{"$ref":"http://127.0.0.1:1/not-requested"}`), 0600)
	os.WriteFile(filepath.Join(dir, "templates", "cm.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Release.Name }}\ndata:\n  kube: {{ .Capabilities.KubeVersion.Version | quote }}\n  lookup: {{ (lookup \"v1\" \"Secret\" \"default\" \"x\") | toJson | quote }}\n"), 0600)
	r, e := Render(context.Background(), Options{Chart: dir, Release: "example", Namespace: "ns", Target: "1.32.0"})
	if e != nil || len(r.Set.Resources) != 1 {
		t.Fatal(r, e)
	}
	if r.Set.Resources[0].Object["metadata"].(map[string]any)["name"] != "example" {
		t.Fatal(r)
	}
	if len(r.Unchecked) == 0 {
		t.Fatal("missing coverage")
	}
}
