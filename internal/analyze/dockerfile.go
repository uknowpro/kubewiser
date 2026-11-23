package analyze

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/uknowpro/kubewiser/internal/manifest"
)

// AttachDockerfile reviews declarations only. It never builds, resolves images,
// expands build arguments, follows COPY paths, or associates a stage with a Pod.
func AttachDockerfile(p *Plan, path string) error {
	b, err := manifest.ReadBounded(path, 1<<20)
	if err != nil {
		return err
	}
	if !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
		return fmt.Errorf("Dockerfile must be UTF-8 text")
	}
	sum := sha256.Sum256(b)
	provenance := fmt.Sprintf("Local Dockerfile sha256:%x; stage declaration only, not deployed-image evidence", sum)
	add := func(line, stage int, id, current, target, status, evidence string) {
		resource := fmt.Sprintf("%s#stage %d, line %d", path, stage, line)
		action := "Verify the built image digest and actual runtime; test memory limits/OOM and CPU throttling on cgroup v2"
		p.ApplicationSources = append(p.ApplicationSources, MatrixRow{Topic: "dockerfile", Resource: resource, Current: current, Target: target, Status: status, Evidence: provenance + "; " + evidence, Action: action})
		p.Actions = append(p.Actions, Action{ID: id, Resource: resource, Requirement: "conditional", Current: current, Recommended: target, Message: evidence, Verification: action, Source: cgroupSource})
	}
	text := strings.TrimPrefix(string(b), "\ufeff")
	// These constructs need a full build frontend. Do not interpret heredoc body
	// text as instructions or silently guess the effect of a custom frontend.
	for i, line := range strings.Split(text, "\n") {
		s := strings.TrimSpace(line)
		lower := strings.ToLower(s)
		unsupported := strings.Contains(s, "<<") && !strings.HasPrefix(s, "#")
		if strings.HasPrefix(lower, "# escape=") && strings.TrimSpace(strings.TrimPrefix(lower, "# escape=")) != "\\" {
			unsupported = true
		}
		if strings.HasPrefix(lower, "# syntax=") {
			frontend := strings.TrimSpace(strings.TrimPrefix(lower, "# syntax="))
			if frontend != "docker/dockerfile:1" {
				unsupported = true
			}
		}
		if unsupported {
			add(i+1, 0, "KW-DOCKERFILE-COVERAGE", "Unsupported Dockerfile syntax", "Review with the actual build frontend", "analyzer_unsupported", "Heredoc, custom frontend or escape directive prevents reliable instruction inspection; contents omitted")
			return nil
		}
	}
	stage, count, start := 0, 0, 0
	pending := ""
	for i, line := range strings.Split(text, "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if pending == "" {
			start = i + 1
		}
		if strings.HasSuffix(s, "\\") {
			pending += strings.TrimSuffix(s, "\\") + " "
			continue
		}
		s = pending + s
		pending = ""
		count++
		if count > 1000 {
			return fmt.Errorf("Dockerfile instruction limit exceeded (1000)")
		}
		fields := strings.Fields(s)
		op := strings.ToUpper(fields[0])
		args := strings.TrimSpace(strings.TrimPrefix(s, fields[0]))
		if op == "FROM" {
			stage++
			image := ""
			for _, f := range fields[1:] {
				if !strings.HasPrefix(f, "--") {
					image = f
					break
				}
			}
			current, target, status, reason := dockerJavaBaseline(image)
			add(start, stage, "KW-DOCKERFILE-BASE", current, target, status, reason)
		}
		switch op {
		case "RUN", "CMD", "ENTRYPOINT", "ENV", "ARG", "HEALTHCHECK", "ONBUILD":
			for _, token := range []string{"memory.limit_in_bytes", "memory.usage_in_bytes", "cpu.cfs_quota_us", "cpu.cfs_period_us", "cpu.shares"} {
				if strings.Contains(args, token) {
					add(start, stage, "KW-DOCKERFILE-V1-REFERENCE", "cgroup v1 file reference: "+token, "Use hierarchy-aware readers; v2 memory.max/memory.current/cpu.max/cpu.weight have different semantics", "review_required", "Static token match; a comment, fallback or unused build stage is possible; command contents omitted")
				}
			}
			if strings.Contains(args, "-XX:-UseContainerSupport") {
				add(start, stage, "KW-DOCKERFILE-JAVA-OPTIONS", "Java container awareness disabled", "Review effective JVM flags and resource sizing on cgroup v2", "review_required", "Static JVM option; build-time and effective runtime configuration may differ; command contents omitted")
			}
		}
		if op == "COPY" || op == "ADD" || op == "RUN" || op == "ONBUILD" {
			// One coverage row per file is enough; do not dump commands or local secrets.
			if !hasDockerfileCoverage(p.ApplicationSources, path) {
				add(start, stage, "KW-DOCKERFILE-CONTENTS", "Image contents not resolved", "Inspect installed/copied runtimes, application dependencies and startup scripts", "input_missing", "COPY/ADD/RUN/ONBUILD may change image contents; source files, package installations, remote URLs and inherited image configuration are not inspected")
			}
		}
	}
	if pending != "" {
		return fmt.Errorf("Dockerfile has an unfinished continuation")
	}
	if stage == 0 {
		return fmt.Errorf("Dockerfile requires a FROM instruction")
	}
	p.Unchecked = append(p.Unchecked, "Dockerfile review inspects all declared stages, including unused build stages. Build target, ARG expansion, inherited configuration, COPY contents, installed packages, registry image contents and deployment digest linkage are unverified. No build, pull, script, network or cluster operation is executed. Dockerfile findings are not part of --after resolution tracking.")
	return nil
}

func hasDockerfileCoverage(rows []MatrixRow, path string) bool {
	for _, r := range rows {
		if strings.HasPrefix(r.Resource, path+"#") && r.Current == "Image contents not resolved" {
			return true
		}
	}
	return false
}

var dockerJavaTag = regexp.MustCompile(`^(\d+)(?:u(\d+)|\.(\d+)\.(\d+))?(?:[-+_].*)?$`)

func dockerJavaBaseline(image string) (current, target, status, reason string) {
	target = "Establish the actual runtime's cgroup v2 support"
	status = "input_missing"
	reason = "Unrecognized/private image, unresolved ARG, stage alias, digest-only reference or ambiguous runtime tag; no compatibility verdict"
	current = "Base runtime unknown (image reference omitted)"
	if strings.ContainsAny(image, "$@") {
		return
	}
	ref := strings.TrimPrefix(strings.TrimPrefix(image, "docker.io/"), "library/")
	repo, tag, ok := strings.Cut(ref, ":")
	if !ok || (repo != "openjdk" && repo != "eclipse-temurin") {
		return
	}
	m := dockerJavaTag.FindStringSubmatch(tag)
	if m == nil || strings.Contains(strings.ToLower(tag), "openj9") {
		return
	}
	major, _ := strconv.Atoi(m[1])
	current = fmt.Sprintf("Recognized HotSpot image tag declares Java %s", m[1])
	target = "HotSpot cgroup v2 baseline: 8u372+, 11.0.16+, or 15+; use a maintained vendor build"
	supported, known := false, false
	switch {
	case major == 8 && m[2] != "":
		patch, _ := strconv.Atoi(m[2])
		supported, known = patch >= 372, true
		current += "u" + m[2]
	case major == 11 && m[3] != "":
		minor, _ := strconv.Atoi(m[3])
		patch, _ := strconv.Atoi(m[4])
		supported, known = minor > 0 || patch >= 16, true
		current += "." + m[3] + "." + m[4]
	case major >= 15:
		supported, known = true, true
	case major > 0 && major != 8 && major != 11:
		known = true
	}
	if !known {
		reason = "Java tag omits the update version; floating tags cannot establish the documented baseline"
		return
	}
	if supported {
		status = "execution_required"
		reason = "Declared tag meets a selected feature baseline only; mutable tags, vendor patches, later installs and actual image contents are unverified. This is not a compatibility or maintenance approval"
	} else {
		status = "review_required"
		reason = "Declared tag predates the documented HotSpot cgroup v2 baseline; runtime may read host limits and size resources incorrectly. Verify vendor backports and the actual built image"
	}
	return
}
