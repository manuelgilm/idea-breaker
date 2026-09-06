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
	return parsePersona(data, path)
}

func parsePersona(data []byte, source string) (engine.Persona, error) {
	var fp filePersona
	if err := yaml.Unmarshal(data, &fp); err != nil {
		return engine.Persona{}, fmt.Errorf("parse %q: %w", source, err)
	}
	if fp.Name == "" {
		return engine.Persona{}, fmt.Errorf("persona file %q has no name", source)
	}
	if fp.Prompt == "" {
		return engine.Persona{}, fmt.Errorf("persona file %q has no prompt", source)
	}
	return engine.Persona{Name: fp.Name, SystemPrompt: fp.Prompt}, nil
}
