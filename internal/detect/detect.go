// Package detect identifies the technology stacks present in a project directory.
// Detection is based solely on the presence and content of manifest files;
// it never executes project code or makes network requests.
package detect

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Stack describes one detected technology layer.
type Stack struct {
	Language  string `json:"language"`            // e.g. "node", "php", "python", "go", "rust"
	Framework string `json:"framework,omitempty"` // e.g. "nextjs", "laravel", "" if unknown
	Manifest  string `json:"manifest"`            // relative path to the primary manifest for this stack
}

// Result holds the full detection outcome for a project directory.
type Result struct {
	Root       string
	Stacks     []Stack
	PrimaryIdx int
}

// Primary returns the primary detected stack, or the zero value if none.
func (r *Result) Primary() (Stack, bool) {
	if len(r.Stacks) == 0 {
		return Stack{}, false
	}
	return r.Stacks[r.PrimaryIdx], true
}

// SubProjectResult is one detected sub-project found during a monorepo walk.
type SubProjectResult struct {
	Path   string // relative path from scan root; "." means the root itself
	Stacks []Stack
}

// monorepoSkipDirs lists directory names that DetectAll will never descend into.
var monorepoSkipDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".git":         true,
	"dist":         true,
	"build":        true,
	".next":        true,
	"target":       true,
	"__pycache__":  true,
	"venv":         true,
	".venv":        true,
}

// DetectAll walks root up to maxDepth directory levels, finds every
// sub-directory that contains at least one recognised manifest, and returns
// one SubProjectResult per such directory. Results are sorted by Path.
// When maxDepth == 0 DetectAll behaves like Detect (root only).
func DetectAll(root string, maxDepth int) ([]SubProjectResult, error) {
	var results []SubProjectResult
	err := walkDetect(root, root, 0, maxDepth, &results)
	if err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})
	return results, nil
}

func walkDetect(scanRoot, dir string, depth, maxDepth int, results *[]SubProjectResult) error {
	res, err := Detect(dir)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(scanRoot, dir)
	if err != nil {
		rel = dir
	}
	if rel == "" {
		rel = "."
	}

	if len(res.Stacks) > 0 {
		*results = append(*results, SubProjectResult{Path: rel, Stacks: res.Stacks})
	}

	if depth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // skip unreadable directories silently
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if monorepoSkipDirs[name] || strings.HasPrefix(name, ".") {
			continue
		}
		if err := walkDetect(scanRoot, filepath.Join(dir, name), depth+1, maxDepth, results); err != nil {
			return err
		}
	}
	return nil
}

// Detect analyzes the directory at root and returns the detected stacks.
func Detect(root string) (*Result, error) {
	type probe struct {
		filename string
		fn       func(root, fp string) Stack
	}

	probes := []probe{
		{"package.json", probeNode},
		{"composer.json", probePHP},
		{"go.mod", probeGo},
		{"Cargo.toml", probeRust},
		{"pyproject.toml", probePyproject},
		{"requirements.txt", probePythonRequirements},
		{"Pipfile", probePipfile},
	}

	seen := map[string]bool{}
	var stacks []Stack

	for _, p := range probes {
		fp := filepath.Join(root, p.filename)
		if _, err := os.Stat(fp); err != nil {
			continue
		}
		s := p.fn(root, fp)
		if seen[s.Language] {
			continue
		}
		seen[s.Language] = true
		s.Manifest = p.filename
		stacks = append(stacks, s)
	}

	sort.Slice(stacks, func(i, j int) bool {
		return stacks[i].Language < stacks[j].Language
	})

	return &Result{Root: root, Stacks: stacks, PrimaryIdx: 0}, nil
}

// ─── per-ecosystem probes ────────────────────────────────────────────────────

func probeNode(root, fp string) Stack {
	s := Stack{Language: "node"}
	data, err := os.ReadFile(fp)
	if err != nil {
		return s
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return s
	}

	all := make(map[string]bool)
	for k := range pkg.Dependencies {
		all[k] = true
	}
	for k := range pkg.DevDependencies {
		all[k] = true
	}

	switch {
	case all["next"]:
		s.Framework = "nextjs"
	case all["nuxt"] || all["nuxt3"]:
		s.Framework = "nuxt"
	case all["@nestjs/core"]:
		s.Framework = "nestjs"
	case all["express"]:
		s.Framework = "express"
	case all["fastify"]:
		s.Framework = "fastify"
	case all["vite"]:
		s.Framework = "vite"
	case all["react"]:
		s.Framework = "react"
	case all["vue"]:
		s.Framework = "vue"
	case all["svelte"]:
		s.Framework = "svelte"
	}
	return s
}

func probePHP(root, fp string) Stack {
	s := Stack{Language: "php"}
	artisan := filepath.Join(root, "artisan")
	if _, err := os.Stat(artisan); err == nil {
		s.Framework = "laravel"
		return s
	}
	data, err := os.ReadFile(fp)
	if err != nil {
		return s
	}
	var comp struct {
		Require map[string]string `json:"require"`
	}
	if err := json.Unmarshal(data, &comp); err != nil {
		return s
	}
	if comp.Require["laravel/framework"] != "" {
		s.Framework = "laravel"
	} else if comp.Require["symfony/symfony"] != "" || comp.Require["symfony/framework-bundle"] != "" {
		s.Framework = "symfony"
	}
	return s
}

func probeGo(_, fp string) Stack   { return Stack{Language: "go"} }
func probeRust(_, fp string) Stack { return Stack{Language: "rust"} }

func probePyproject(root, fp string) Stack {
	s := Stack{Language: "python"}
	data, err := os.ReadFile(fp)
	if err != nil {
		return s
	}
	content := strings.ToLower(string(data))
	switch {
	case strings.Contains(content, "django"):
		s.Framework = "django"
	case strings.Contains(content, "flask"):
		s.Framework = "flask"
	case strings.Contains(content, "fastapi"):
		s.Framework = "fastapi"
	}
	return s
}

func probePythonRequirements(root, fp string) Stack {
	s := Stack{Language: "python"}
	data, err := os.ReadFile(fp)
	if err != nil {
		return s
	}
	content := strings.ToLower(string(data))
	switch {
	case containsLine(content, "django"):
		s.Framework = "django"
	case containsLine(content, "flask"):
		s.Framework = "flask"
	case containsLine(content, "fastapi"):
		s.Framework = "fastapi"
	}
	return s
}

func probePipfile(root, fp string) Stack {
	s := Stack{Language: "python"}
	data, err := os.ReadFile(fp)
	if err != nil {
		return s
	}
	content := strings.ToLower(string(data))
	switch {
	case strings.Contains(content, "django"):
		s.Framework = "django"
	case strings.Contains(content, "flask"):
		s.Framework = "flask"
	case strings.Contains(content, "fastapi"):
		s.Framework = "fastapi"
	}
	return s
}

func containsLine(content, prefix string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return true
		}
	}
	return false
}

// IsBinary reports whether data looks like a binary file.
func IsBinary(data []byte) bool {
	check := data
	if len(check) > 512 {
		check = check[:512]
	}
	return bytes.IndexByte(check, 0) >= 0
}
