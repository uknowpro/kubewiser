package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/uknowpro/kubewiser/internal/manifest"
	ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structural "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	cel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	kubejson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

// CELReview uses the pinned Kubernetes implementation, not a home-grown evaluator.
// It evaluates supplied CRD rules only; admission webhooks and server defaulting
// are deliberately not simulated. Old values must be supplied explicitly.
func CELReview(ctx context.Context, set manifest.Set, old []manifest.Resource) (Plan, error) {
	p := Plan{Kind: "crd-cel", Status: "partial", From: "local CRDs", To: "Kubernetes CEL library v0.37.2", Actions: []Action{}, Steps: []string{}, Unchecked: []string{"Pinned Kubernetes CEL library v0.37.2 evaluates supplied structural CRD rules. This is not a target-server feature-gate or admission guarantee.", "No pruning/defaulting, webhook conversion, admission webhooks or mutation occurs. Ratcheting is not enabled. Validation error values and user messages are omitted."}}
	if len(set.Issues) > 0 {
		return p, fmt.Errorf("CEL inputs contain parse errors")
	}
	type compiled struct {
		s          *structural.Structural
		v          *cel.Validator
		transition bool
	}
	schemas := map[string]compiled{}
	rules := 0
	for _, r := range set.Resources {
		if e := ctx.Err(); e != nil {
			return p, e
		}
		if manifest.String(r.Object, "kind") != "CustomResourceDefinition" {
			continue
		}
		spec := manifest.Map(r.Object["spec"])
		group := manifest.String(spec, "group")
		kind := manifest.String(manifest.Map(spec["names"]), "kind")
		for _, rawVersion := range manifest.Slice(spec["versions"]) {
			ver := manifest.Map(rawVersion)
			if ver["served"] == false {
				continue
			}
			raw := manifest.Map(manifest.At(ver, "schema", "openAPIV3Schema"))
			if len(raw) == 0 {
				continue
			}
			transition := false
			var inspect func(any, int) error
			inspect = func(x any, depth int) error {
				if depth > 100 {
					return fmt.Errorf("CEL schema depth limit exceeded")
				}
				switch n := x.(type) {
				case map[string]any:
					for _, v := range manifest.Slice(n["x-kubernetes-validations"]) {
						rule := manifest.Map(v)
						expression := manifest.String(rule, "rule")
						rules++
						if rules > 200 || len(expression) > 4096 {
							return fmt.Errorf("CEL rule limit exceeded")
						}
						if strings.Contains(expression, "oldSelf") {
							transition = true
						}
					}
					for _, v := range n {
						if e := inspect(v, depth+1); e != nil {
							return e
						}
					}
				case []any:
					for _, v := range n {
						if e := inspect(v, depth+1); e != nil {
							return e
						}
					}
				}
				return nil
			}
			if e := inspect(raw, 0); e != nil {
				return p, e
			}
			b, _ := json.Marshal(raw)
			if len(b) > 1<<20 {
				return p, fmt.Errorf("CEL schema size limit exceeded")
			}
			var v1 extv1.JSONSchemaProps
			if e := json.Unmarshal(b, &v1); e != nil {
				return p, fmt.Errorf("invalid CRD schema")
			}
			var native ext.JSONSchemaProps
			if e := extv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(&v1, &native, nil); e != nil {
				return p, fmt.Errorf("invalid CRD schema conversion")
			}
			s, e := structural.NewStructural(&native)
			if e != nil || len(structural.ValidateStructural(field.NewPath("schema"), s)) > 0 {
				return p, fmt.Errorf("CEL requires a valid structural CRD schema")
			}
			k := group + "/" + manifest.String(ver, "name") + "|" + kind
			if _, ok := schemas[k]; ok {
				return p, fmt.Errorf("duplicate CRD schema")
			}
			if len(schemas) >= 50 {
				return p, fmt.Errorf("CEL schema count limit exceeded")
			}
			schemas[k] = compiled{s, cel.NewValidator(s, true, celconfig.PerCallLimit), transition}
		}
	}
	previous := map[string]manifest.Resource{}
	for _, r := range old {
		if _, ok := previous[r.ID()]; ok {
			return p, fmt.Errorf("duplicate old resource identity")
		}
		previous[r.ID()] = r
	}
	checked := 0
	for _, r := range set.Resources {
		if e := ctx.Err(); e != nil {
			return p, e
		}
		c, ok := schemas[schemaKey(r)]
		if !ok || c.v == nil {
			continue
		}
		if impreciseNumber(r.Object) {
			p.Unchecked = append(p.Unchecked, "CEL skipped: input contains numbers outside exact JSON float64 integer precision for "+r.ID())
			continue
		}
		var before any
		if prev, ok := previous[r.ID()]; ok {
			if impreciseNumber(prev.Object) {
				p.Unchecked = append(p.Unchecked, "CEL skipped: old object has numbers outside exact JSON float64 integer precision for "+r.ID())
				continue
			}
			before = celObject(prev.Object)
		} else if c.transition {
			p.Unchecked = append(p.Unchecked, "No matching old object for transition rules: "+r.ID())
		}
		checked++
		errs, budget := c.v.Validate(ctx, field.NewPath("resource"), c.s, celObject(r.Object), before, 1000000)
		for _, e := range errs {
			p.Actions = append(p.Actions, Action{ID: "KW-CEL", Resource: r.ID(), Requirement: "required", Message: "CRD CEL constraint or rule compilation failed at " + e.Field + " (" + string(e.Type) + ")", Verification: "Review the local x-kubernetes-validations rule and test with the target API server; raw values and custom messages are omitted", Source: "https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules"})
		}
		if budget < 0 {
			p.Unchecked = append(p.Unchecked, "CEL cost limit exhausted for "+r.ID())
		}
	}
	p.Steps = append(p.Steps, fmt.Sprintf("Evaluated CEL rule sets for %d supplied resources; %d rules declared", checked, rules))
	if checked == 0 {
		p.Unchecked = append(p.Unchecked, "No matching resources with CEL validation rules were evaluated")
	}
	return p, nil
}

func celObject(obj map[string]any) map[string]any {
	b, _ := json.Marshal(obj)
	var out map[string]any
	_ = kubejson.Unmarshal(b, &out)
	return out
}

func impreciseNumber(value any) bool {
	switch v := value.(type) {
	case float64:
		return math.Abs(v) >= 9007199254740992
	case map[string]any:
		for _, child := range v {
			if impreciseNumber(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if impreciseNumber(child) {
				return true
			}
		}
	}
	return false
}
