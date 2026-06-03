package skills_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CaowardlyLion/OpenHome/internal/skills"
)

func TestLoadCatalog(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "planning", "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "INDEX.md"), []byte("# Root"), 0o644)
	os.WriteFile(filepath.Join(root, "planning", "INDEX.md"), []byte("# Planning"), 0o644)
	os.WriteFile(filepath.Join(root, "planning", "tasks", "SKILL.md"), []byte("---\nname: tasks\ndescription: Make tasks\nallowedTools:\n  - writeFile\n---\n# Tasks"), 0o644)
	catalog, err := skills.Load(root, []string{"writeFile"})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) != 1 || catalog.Skills[0].Name != "tasks" || !strings.Contains(catalog.LibrarianContext, "planning/INDEX.md") {
		t.Fatalf("catalog = %#v", catalog)
	}
}

func TestLoadCatalogRejectsUnknownTool(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "bad"), 0o755)
	os.WriteFile(filepath.Join(root, "bad", "SKILL.md"), []byte("---\nname: bad\ndescription: Bad\nallowedTools:\n  - runShell\n---\n# Bad"), 0o644)
	if _, err := skills.Load(root, []string{"writeFile"}); err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("error = %v", err)
	}
}

func TestRepositoryWebSearchSkillLoads(t *testing.T) {
	catalog, err := skills.Load(filepath.Join("..", "..", "skills"), []string{
		"listFiles", "readFile", "searchFiles", "writeFile", "webSearch", "fetchURL", "browserInteract", "sendEmail",
	})
	if err != nil {
		t.Fatal(err)
	}
	skill, err := catalog.Find("web-search")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(skill.Content, "Treat search titles and snippets as") || !strings.Contains(skill.Content, "`fetchURL`") {
		t.Fatalf("skill content = %q", skill.Content)
	}
}

func TestRepositoryCostcoPriceMatchingSkillLoads(t *testing.T) {
	catalog, err := skills.Load(filepath.Join("..", "..", "skills"), []string{
		"listFiles", "readFile", "searchFiles", "writeFile", "webSearch", "fetchURL", "browserInteract", "sendEmail",
	})
	if err != nil {
		t.Fatal(err)
	}
	skill, err := catalog.Find("costco-price-matching")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"browser-navigation", "If the user did not provide a location", "nearby warehouses"} {
		if !strings.Contains(skill.Content, expected) {
			t.Fatalf("skill missing %q: %q", expected, skill.Content)
		}
	}
}
