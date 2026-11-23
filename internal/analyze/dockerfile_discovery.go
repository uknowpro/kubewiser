package analyze

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AttachDockerfiles uses only the caller-selected source scope. Snapshot paths
// are deliberately not passed here, and parent repositories are never searched.
func AttachDockerfiles(ctx context.Context, p *Plan, source, explicit string) error {
	if explicit != "" {
		return AttachDockerfile(p, explicit)
	}
	missing := func(evidence string) {
		p.ApplicationSources = append(p.ApplicationSources, MatrixRow{Topic: "dockerfile", Resource: source, Current: "Dockerfile evidence incomplete", Target: "Review application runtime and direct cgroup dependencies", Status: "input_missing", Evidence: evidence, Action: "Place Dockerfile/Containerfile in the source scope or supply --dockerfile FILE; validate the built image on cgroup v2"})
	}
	if source == "" {
		missing("Snapshot-only input does not include application build sources; no automatic filesystem search performed")
		return nil
	}
	paths, err := discoverDockerfiles(ctx, source)
	if err != nil {
		missing("Automatic Dockerfile discovery incomplete: " + err.Error())
	}
	if len(paths) == 0 {
		if err == nil {
			missing("No Dockerfile/Containerfile found in source scope; absence is not compatibility evidence")
		}
		return ctx.Err()
	}
	var total int64
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			missing("Discovered Dockerfile is no longer a readable regular file")
			continue
		}
		total += info.Size()
		if total > 8<<20 {
			missing("Automatic Dockerfile input budget exceeded (8 MiB)")
			break
		}
		if err := AttachDockerfile(p, path); err != nil {
			missing(fmt.Sprintf("Could not inspect %s: %s", path, err))
		}
	}
	return ctx.Err()
}

func discoverDockerfiles(ctx context.Context, source string) ([]string, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symbolic-link source is not traversed")
	}
	root := source
	recursive := info.IsDir()
	if !recursive {
		root = filepath.Dir(source)
	}
	files := []string{}
	visited := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		visited++
		if visited > 10000 {
			return fmt.Errorf("entry limit exceeded (10000)")
		}
		if d.IsDir() {
			if path != root {
				switch d.Name() {
				case "node_modules", "vendor", "dist", "build", "target", "__pycache__":
					return filepath.SkipDir
				}
				if !recursive || strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if name == "dockerfile" || name == "containerfile" || strings.HasPrefix(name, "dockerfile.") || strings.HasPrefix(name, "containerfile.") || strings.HasSuffix(name, ".dockerfile") || strings.HasSuffix(name, ".containerfile") {
			if len(files) >= 32 {
				return fmt.Errorf("Dockerfile limit exceeded (32)")
			}
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
