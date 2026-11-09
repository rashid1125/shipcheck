package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func makeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func stackByLang(stacks []Stack, lang string) (Stack, bool) {
	for _, s := range stacks {
		if s.Language == lang {
			return s, true
		}
	}
	return Stack{}, false
}

func TestDetect_NodeJS_Plain(t *testing.T) {
	root := makeProject(t, map[string]string{
		"package.json": `{"name":"test","dependencies":{"express":"^4.0.0"}}`,
	})
	r, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := stackByLang(r.Stacks, "node")
	if !ok {
		t.Fatal("expected node stack")
	}
	if s.Framework != "express" {
		t.Errorf("expected express, got %q", s.Framework)
	}
}

func TestDetect_NodeJS_NextJS(t *testing.T) {
	root := makeProject(t, map[string]string{
		"package.json": `{"dependencies":{"next":"13.0.0","react":"18.0.0"}}`,
	})
	r, _ := Detect(root)
	s, ok := stackByLang(r.Stacks, "node")
	if !ok {
		t.Fatal("expected node stack")
	}
	if s.Framework != "nextjs" {
		t.Errorf("expected nextjs, got %q", s.Framework)
	}
}

func TestDetect_PHP_Laravel(t *testing.T) {
	root := makeProject(t, map[string]string{
		"composer.json": `{"require":{"php":"^8.1"}}`,
		"artisan":       `#!/usr/bin/env php`,
	})
	r, _ := Detect(root)
	s, ok := stackByLang(r.Stacks, "php")
	if !ok {
		t.Fatal("expected php stack")
	}
	if s.Framework != "laravel" {
		t.Errorf("expected laravel, got %q", s.Framework)
	}
}

func TestDetect_PHP_Generic(t *testing.T) {
	root := makeProject(t, map[string]string{
		"composer.json": `{"require":{"php":"^8.0"}}`,
	})
	r, _ := Detect(root)
	s, ok := stackByLang(r.Stacks, "php")
	if !ok {
		t.Fatal("expected php stack")
	}
	if s.Framework != "" {
		t.Errorf("expected no framework, got %q", s.Framework)
	}
}

func TestDetect_Go(t *testing.T) {
	root := makeProject(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.22\n",
	})
	r, _ := Detect(root)
	if _, ok := stackByLang(r.Stacks, "go"); !ok {
		t.Fatal("expected go stack")
	}
}

func TestDetect_Rust(t *testing.T) {
	root := makeProject(t, map[string]string{
		"Cargo.toml": "[package]\nname = \"myapp\"\nversion = \"0.1.0\"\n",
	})
	r, _ := Detect(root)
	if _, ok := stackByLang(r.Stacks, "rust"); !ok {
		t.Fatal("expected rust stack")
	}
}

func TestDetect_Python_Django_Requirements(t *testing.T) {
	root := makeProject(t, map[string]string{
		"requirements.txt": "Django>=4.0\npsycopg2>=2.9\n",
	})
	r, _ := Detect(root)
	s, ok := stackByLang(r.Stacks, "python")
	if !ok {
		t.Fatal("expected python stack")
	}
	if s.Framework != "django" {
		t.Errorf("expected django, got %q", s.Framework)
	}
}

func TestDetect_Python_FastAPI_Pyproject(t *testing.T) {
	root := makeProject(t, map[string]string{
		"pyproject.toml": "[tool.poetry.dependencies]\nfastapi = \"^0.100\"\n",
	})
	r, _ := Detect(root)
	s, ok := stackByLang(r.Stacks, "python")
	if !ok {
		t.Fatal("expected python stack")
	}
	if s.Framework != "fastapi" {
		t.Errorf("expected fastapi, got %q", s.Framework)
	}
}

func TestDetect_MultiStack_Deterministic(t *testing.T) {
	root := makeProject(t, map[string]string{
		"package.json": `{"dependencies":{"express":"^4"}}`,
		"go.mod":       "module example.com/app\ngo 1.22\n",
	})

	// Run multiple times; result must be identical each time.
	var prev []string
	for i := 0; i < 5; i++ {
		r, _ := Detect(root)
		var langs []string
		for _, s := range r.Stacks {
			langs = append(langs, s.Language)
		}
		if prev != nil {
			for j, l := range langs {
				if l != prev[j] {
					t.Fatalf("non-deterministic order at run %d: %v vs %v", i, langs, prev)
				}
			}
		}
		prev = langs
	}
}

func TestDetect_Empty(t *testing.T) {
	root := t.TempDir()
	r, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Stacks) != 0 {
		t.Errorf("expected no stacks, got %v", r.Stacks)
	}
}

func TestDetect_InvalidManifest(t *testing.T) {
	root := makeProject(t, map[string]string{
		"package.json": `not valid json`,
	})
	r, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	// Should still detect node (just no framework)
	s, ok := stackByLang(r.Stacks, "node")
	if !ok {
		t.Fatal("expected node stack even for invalid JSON")
	}
	if s.Framework != "" {
		t.Errorf("expected no framework for unparseable manifest, got %q", s.Framework)
	}
}

func TestIsBinary(t *testing.T) {
	if IsBinary([]byte("hello world")) {
		t.Error("plain text should not be binary")
	}
	if !IsBinary(append([]byte("data"), 0, 'a', 'b')) {
		t.Error("data with null byte should be binary")
	}
}
