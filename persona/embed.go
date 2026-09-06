package persona

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"

	"github.com/manuelgilm/idea-breaker/engine"
)

//go:embed fixtures/*.yaml
var embeddedFS embed.FS

// synthFilename is the reserved file holding the synthesizer prompt.
// It is excluded from Load so it is never treated as a persona.
const synthFilename = "synthesizer.yaml"

// EmbeddedSource reads personas from YAML files embedded into the binary.
type EmbeddedSource struct{}

// Load reads every embedded fixtures/*.yaml (except synthesizer.yaml) and
// returns the personas in filename order.
func (EmbeddedSource) Load() ([]engine.Persona, error) {
	entries, err := fs.ReadDir(embeddedFS, "fixtures")
	if err != nil {
		return nil, fmt.Errorf("read embedded personas: %w", err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if e.Name() == synthFilename {
			continue
		}
		if ext := path.Ext(e.Name()); ext == ".yaml" || ext == ".yml" {
			paths = append(paths, path.Join("fixtures", e.Name()))
		}
	}
	sort.Strings(paths)

	personas := make([]engine.Persona, 0, len(paths))
	for _, p := range paths {
		pp, err := embeddedPersona(p)
		if err != nil {
			return nil, err
		}
		personas = append(personas, pp)
	}
	return personas, nil
}

// LoadEmbeddedSynthesizer reads the synthesizer prompt from the embedded
// fixtures/synthesizer.yaml.
func LoadEmbeddedSynthesizer() (string, error) {
	p, err := embeddedPersona(path.Join("fixtures", synthFilename))
	if err != nil {
		return "", err
	}
	return p.SystemPrompt, nil
}

func embeddedPersona(p string) (engine.Persona, error) {
	data, err := embeddedFS.ReadFile(p)
	if err != nil {
		return engine.Persona{}, fmt.Errorf("read embedded %q: %w", p, err)
	}
	return parsePersona(data, p)
}
