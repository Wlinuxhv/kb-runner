package cases

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Case struct {
	Name        string            `yaml:"name" json:"name"`
	Path        string            `yaml:"path" json:"path"`
	Language    string            `yaml:"language" json:"language"`
	Category    string            `yaml:"category" json:"category"`
	Tags        []string          `yaml:"tags" json:"tags"`
	Description string            `yaml:"description" json:"description"`
	Params      map[string]string `yaml:"params" json:"params"`
	Timeout     time.Duration     `yaml:"timeout" json:"timeout"`
	Weight      float64           `yaml:"weight" json:"weight"`
}

type FilterOptions struct {
	Category string
	Tags     []string
	Search   string
}

type Manager struct {
	cases map[string]*Case
	mu    sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		cases: make(map[string]*Case),
	}
}

func (m *Manager) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read cases file: %w", err)
	}

	var file struct {
		Cases []*Case `yaml:"cases"`
		Categories []struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		} `yaml:"categories"`
	}

	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("failed to unmarshal cases file: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range file.Cases {
		if c.Name == "" {
			continue
		}
		if c.Timeout == 0 {
			c.Timeout = 300 * time.Second
		}
		if c.Weight == 0 {
			c.Weight = 1.0
		}
		if c.Params == nil {
			c.Params = make(map[string]string)
		}
		if c.Tags == nil {
			c.Tags = []string{}
		}
		m.cases[c.Name] = c
	}

	return nil
}

func (m *Manager) LoadFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := m.LoadFromFile(path); err != nil {
			return fmt.Errorf("failed to load cases from %s: %w", path, err)
		}
	}

	return nil
}

func (m *Manager) Add(c *Case) error {
	if c.Name == "" {
		return fmt.Errorf("case name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cases[c.Name] = c
	return nil
}

func (m *Manager) Get(name string) (*Case, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.cases[name]
	if !ok {
		return nil, fmt.Errorf("case not found: %s", name)
	}
	return c, nil
}

func (m *Manager) List(opts FilterOptions) []*Case {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Case, 0)

	for _, c := range m.cases {
		if !m.matchFilter(c, opts) {
			continue
		}
		result = append(result, c)
	}

	return result
}

func (m *Manager) matchFilter(c *Case, opts FilterOptions) bool {
	if opts.Category != "" && c.Category != opts.Category {
		return false
	}

	if len(opts.Tags) > 0 {
		tagMap := make(map[string]bool)
		for _, t := range c.Tags {
			tagMap[t] = true
		}

		matched := false
		for _, t := range opts.Tags {
			if tagMap[t] {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if opts.Search != "" {
		searchLower := strings.ToLower(opts.Search)
		nameMatch := strings.Contains(strings.ToLower(c.Name), searchLower)
		descMatch := strings.Contains(strings.ToLower(c.Description), searchLower)
		if !nameMatch && !descMatch {
			return false
		}
	}

	return true
}

func (m *Manager) GetByCategory(category string) []*Case {
	return m.List(FilterOptions{Category: category})
}

func (m *Manager) GetByTags(tags ...string) []*Case {
	return m.List(FilterOptions{Tags: tags})
}

func (m *Manager) Search(query string) []*Case {
	return m.List(FilterOptions{Search: query})
}

func (m *Manager) AllCategories() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	categories := make(map[string]bool)
	for _, c := range m.cases {
		if c.Category != "" {
			categories[c.Category] = true
		}
	}

	result := make([]string, 0, len(categories))
	for cat := range categories {
		result = append(result, cat)
	}
	return result
}

func (m *Manager) AllTags() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tags := make(map[string]bool)
	for _, c := range m.cases {
		for _, t := range c.Tags {
			tags[t] = true
		}
	}

	result := make([]string, 0, len(tags))
	for t := range tags {
		result = append(result, t)
	}
	return result
}

func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.cases)
}

func (c *Case) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("case name is required")
	}
	if c.Path == "" {
		return fmt.Errorf("case path is required")
	}
	if c.Language == "" {
		c.Language = "bash"
	}
	return nil
}

func (c *Case) ToJSON() ([]byte, error) {
	return yaml.Marshal(c)
}
