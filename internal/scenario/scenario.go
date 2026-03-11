package scenario

import (
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type ExecutionConfig struct {
	Parallel    bool          `yaml:"parallel"`
	MaxParallel int           `yaml:"max_parallel"`
	Timeout     time.Duration `yaml:"timeout"`
	FailFast    bool          `yaml:"fail_fast"`
}

type Scenario struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	Cases       []string        `yaml:"cases"`
	Execution   ExecutionConfig `yaml:"execution"`
}

type Manager struct {
	scenarios map[string]*Scenario
	mu        sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		scenarios: make(map[string]*Scenario),
	}
}

func (m *Manager) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read scenario file: %w", err)
	}

	var file struct {
		Scenarios []*Scenario `yaml:"scenarios"`
	}

	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("failed to unmarshal scenario file: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range file.Scenarios {
		m.scenarios[s.Name] = s
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

		name := entry.Name()
		ext := name[len(name)-5:]
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := dir + string(os.PathSeparator) + name
		if err := m.LoadFromFile(path); err != nil {
			return fmt.Errorf("failed to load scenario from %s: %w", path, err)
		}
	}

	return nil
}

func (m *Manager) Get(name string) (*Scenario, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.scenarios[name]
	if !ok {
		return nil, fmt.Errorf("scenario not found: %s", name)
	}
	return s, nil
}

func (m *Manager) List() []*Scenario {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Scenario, 0, len(m.scenarios))
	for _, s := range m.scenarios {
		result = append(result, s)
	}
	return result
}

func (m *Manager) Add(s *Scenario) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.scenarios[s.Name]; ok {
		return fmt.Errorf("scenario already exists: %s", s.Name)
	}

	m.scenarios[s.Name] = s
	return nil
}

func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.scenarios[name]; !ok {
		return fmt.Errorf("scenario not found: %s", name)
	}

	delete(m.scenarios, name)
	return nil
}

func (s *Scenario) ToJSON() ([]byte, error) {
	return yaml.Marshal(s)
}

func (s *Scenario) CaseCount() int {
	return len(s.Cases)
}

func (s *Scenario) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("scenario name is required")
	}

	if len(s.Cases) == 0 {
		return fmt.Errorf("scenario must have at least one case")
	}

	if s.Execution.Timeout < 0 {
		return fmt.Errorf("execution timeout must be non-negative")
	}

	if s.Execution.MaxParallel < 0 {
		return fmt.Errorf("max parallel must be non-negative")
	}

	return nil
}

func DefaultExecutionConfig() ExecutionConfig {
	return ExecutionConfig{
		Parallel:    false,
		MaxParallel: 1,
		Timeout:     300 * time.Second,
		FailFast:    false,
	}
}
