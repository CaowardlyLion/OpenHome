package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name         string
	Description  string
	AllowedTools []string
	Content      string
	FilePath     string
	RelativePath string
}

type Catalog struct {
	IndexContent     string
	Skills           []Skill
	LibrarianContext string
}

type frontmatter struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	AllowedTools []string `yaml:"allowedTools"`
}

func Load(dir string, availableTools []string) (Catalog, error) {
	var indexes, skillFiles []string
	err := filepath.WalkDir(dir, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch entry.Name() {
		case "INDEX.md":
			indexes = append(indexes, name)
		case "SKILL.md":
			skillFiles = append(skillFiles, name)
		}
		return nil
	})
	if err != nil {
		return Catalog{}, err
	}
	sort.Strings(indexes)
	sort.Strings(skillFiles)
	var index strings.Builder
	for _, file := range indexes {
		content, err := os.ReadFile(file)
		if err != nil {
			return Catalog{}, err
		}
		relative, _ := filepath.Rel(dir, file)
		fmt.Fprintf(&index, "## %s\n%s\n\n", relative, content)
	}
	var loaded []Skill
	seen := map[string]bool{}
	for _, file := range skillFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			return Catalog{}, err
		}
		meta, body, err := parseFrontmatter(string(content))
		if err != nil {
			return Catalog{}, fmt.Errorf("skill %s: %w", file, err)
		}
		if meta.Name == "" || meta.Description == "" {
			return Catalog{}, fmt.Errorf("skill %s must define name and description", file)
		}
		if seen[meta.Name] {
			return Catalog{}, fmt.Errorf("duplicate skill name: %s", meta.Name)
		}
		seen[meta.Name] = true
		for _, tool := range meta.AllowedTools {
			if !contains(availableTools, tool) {
				return Catalog{}, fmt.Errorf("skill %s references unknown tool: %s", file, tool)
			}
		}
		relative, _ := filepath.Rel(dir, file)
		loaded = append(loaded, Skill{
			Name: meta.Name, Description: meta.Description, AllowedTools: meta.AllowedTools,
			Content: strings.TrimSpace(body), FilePath: file, RelativePath: relative,
		})
	}
	var summaries strings.Builder
	for _, skill := range loaded {
		fmt.Fprintf(&summaries, "- %s (%s): %s; tools=%s\n", skill.Name, skill.RelativePath, skill.Description, strings.Join(skill.AllowedTools, ","))
	}
	indexContent := strings.TrimSpace(index.String())
	return Catalog{IndexContent: indexContent, Skills: loaded, LibrarianContext: indexContent + "\n\n## Skill Summaries\n" + summaries.String()}, nil
}

func (c Catalog) Find(name string) (Skill, error) {
	for _, skill := range c.Skills {
		if skill.Name == name {
			return skill, nil
		}
	}
	return Skill{}, fmt.Errorf("librarian chose unknown skill: %s", name)
}

func parseFrontmatter(content string) (frontmatter, string, error) {
	if !strings.HasPrefix(content, "---\n") {
		return frontmatter{}, "", fmt.Errorf("missing YAML frontmatter")
	}
	parts := strings.SplitN(strings.TrimPrefix(content, "---\n"), "\n---\n", 2)
	if len(parts) != 2 {
		return frontmatter{}, "", fmt.Errorf("unterminated YAML frontmatter")
	}
	var meta frontmatter
	if err := yaml.Unmarshal([]byte(parts[0]), &meta); err != nil {
		return frontmatter{}, "", err
	}
	return meta, parts[1], nil
}

func contains(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}
