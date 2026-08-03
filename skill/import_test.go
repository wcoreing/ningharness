package skill

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportZipFlat(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "pack.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"SKILL.md": "---\nname: humanizer\ndescription: demo skill\n---\n\n# Hi\n",
		"references/a.md": "# ref\n",
		"_meta.json":      `{"slug":"unclecheng-x","version":"1.0.4"}`,
	}); err != nil {
		t.Fatal(err)
	}

	info, written, err := ImportZip(root, zipPath, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "humanizer" {
		t.Fatalf("id=%q", info.ID)
	}
	if info.Name != "humanizer" {
		t.Fatalf("name=%q", info.Name)
	}
	body, err := os.ReadFile(filepath.Join(root, "system", "skills", "humanizer", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "name: humanizer") {
		t.Fatalf("body=%s", body)
	}
	if _, err := os.Stat(filepath.Join(root, "system", "skills", "humanizer", "references", "a.md")); err != nil {
		t.Fatal(err)
	}
	if len(written) < 2 {
		t.Fatalf("written=%v", written)
	}

	_, _, err = ImportZip(root, zipPath, "", false)
	if err == nil {
		t.Fatal("expected exists error")
	}
	_, _, err = ImportZip(root, zipPath, "", true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestImportZipNested(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "nested.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"pack/SKILL.md": "---\nname: nested-skill\ndescription: n\n---\n\n# N\n",
	}); err != nil {
		t.Fatal(err)
	}
	info, _, err := ImportZip(root, zipPath, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "nested-skill" {
		t.Fatalf("id=%q", info.ID)
	}
}

func writeTestZip(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, body := range files {
		fw, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			return err
		}
	}
	return w.Close()
}
