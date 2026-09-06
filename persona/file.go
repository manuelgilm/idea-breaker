package persona

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/manuelgilm/idea-breaker/engine"
	"go.yaml.in/yaml/v3"
)

// filePersona is the on-disk shape of a persona YAML file.
type filePersona struct {
	Name   string `yaml:"name"`
	Prompt string `yaml:"prompt"`
}

// FileSource reads personas from a directory of YAML files, one per persona.
type FileSource struct {
	Dir string
}

// synthFilename is the reserved persona-dir file holding the synthesizer prompt.
// It is excluded from Load so it is never treated as a persona.
const synthFilename = "synthesizer.yaml"

// Load reads every *.yaml file in Dir (except synthesizer.yaml) and returns the
// personas in filename order.
func (s FileSource) Load() ([]engine.Persona, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, fmt.Errorf("read persona dir %q: %w", s.Dir, err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if e.Name() == synthFilename {
			continue
		}
		if ext := filepath.Ext(e.Name()); ext == ".yaml" || ext == ".yml" {
			paths = append(paths, filepath.Join(s.Dir, e.Name()))
		}
	}
	sort.Strings(paths)

	personas := make([]engine.Persona, 0, len(paths))
	for _, path := range paths {
		p, err := loadFile(path)
		if err != nil {
			return nil, err
		}
		personas = append(personas, p)
	}
	return personas, nil
}

// LoadSynthesizer reads the synthesizer prompt from {dir}/synthesizer.yaml.
func LoadSynthesizer(dir string) (string, error) {
	p, err := loadFile(filepath.Join(dir, synthFilename))
	if err != nil {
		return "", err
	}
	return p.SystemPrompt, nil
}

func loadFile(path string) (engine.Persona, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return engine.Persona{}, fmt.Errorf("read %q: %w", path, err)
	}

	var fp filePersona
	if err := yaml.Unmarshal(data, &fp); err != nil {
		return engine.Persona{}, fmt.Errorf("parse %q: %w", path, err)
	}
	if fp.Name == "" {
		return engine.Persona{}, fmt.Errorf("persona file %q has no name", path)
	}
	if fp.Prompt == "" {
		return engine.Persona{}, fmt.Errorf("persona file %q has no prompt", path)
	}
	return engine.Persona{Name: fp.Name, SystemPrompt: fp.Prompt}, nil
}
