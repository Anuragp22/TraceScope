package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds per-project TraceScope settings.
type Config struct {
	Ignore    []string       `yaml:"ignore"`     // glob patterns to exclude files
	MaxDepth  int            `yaml:"max_depth"`   // blast radius depth
	Format    string         `yaml:"format"`      // output format: terminal or json
	TopN      int            `yaml:"top"`         // max results to show
	GraphPath string         `yaml:"graph_path"`  // custom path for graph.json
	Risk      RiskThresholds `yaml:"risk"`        // custom risk thresholds
}

// RiskThresholds controls when functions are classified as HIGH/MEDIUM risk.
type RiskThresholds struct {
	HighCallers         int `yaml:"high_callers"`          // callers for HIGH (default 10)
	HighExportedCallers int `yaml:"high_exported_callers"` // callers for HIGH when exported (default 5)
	MediumCallers       int `yaml:"medium_callers"`        // callers for MEDIUM (default 3)
}

// Defaults returns a Config with all default values applied.
func Defaults() Config {
	return Config{
		MaxDepth: 5,
		Format:   "terminal",
		Risk: RiskThresholds{
			HighCallers:         10,
			HighExportedCallers: 5,
			MediumCallers:       3,
		},
	}
}

// Load searches for .tracescope.yaml or .tracescope.yml starting from dir
// and walking up to the filesystem root. Returns defaults if no file found.
func Load(dir string) (Config, string, error) {
	cfg := Defaults()

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return cfg, "", nil
	}

	configPath := findConfigFile(absDir)
	if configPath == "" {
		return cfg, "", nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, configPath, err
	}

	var fileCfg fileConfig
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return cfg, configPath, err
	}

	warnings := fileCfg.merge(&cfg, filepath.Dir(configPath))
	return cfg, configPath, joinWarnings(warnings)
}

// fileConfig mirrors Config with pointers, so that a key written in the file is
// distinguishable from a key left out. The previous version merged only
// non-zero values, which meant `max_depth: 0` and `high_callers: 0` were
// silently discarded and the user was told nothing.
type fileConfig struct {
	Ignore    []string  `yaml:"ignore"`
	MaxDepth  *int      `yaml:"max_depth"`
	Format    *string   `yaml:"format"`
	TopN      *int      `yaml:"top"`
	GraphPath *string   `yaml:"graph_path"`
	Risk      *fileRisk `yaml:"risk"`
}

type fileRisk struct {
	HighCallers         *int `yaml:"high_callers"`
	HighExportedCallers *int `yaml:"high_exported_callers"`
	MediumCallers       *int `yaml:"medium_callers"`
}

// merge applies the file over the defaults, returning a message for every key
// that was present but unusable. Out-of-range values keep the default rather
// than taking effect, but the caller is told which ones and why.
func (f fileConfig) merge(cfg *Config, configDir string) []string {
	var warn []string

	atLeast := func(name string, v *int, min int, dst *int) {
		if v == nil {
			return
		}
		if *v < min {
			warn = append(warn, fmt.Sprintf("%s: %d is below the minimum of %d; keeping %d", name, *v, min, *dst))
			return
		}
		*dst = *v
	}

	atLeast("max_depth", f.MaxDepth, 1, &cfg.MaxDepth)
	atLeast("top", f.TopN, 0, &cfg.TopN) // 0 is meaningful here: show everything

	if f.Format != nil {
		switch *f.Format {
		case "terminal", "json":
			cfg.Format = *f.Format
		default:
			warn = append(warn, fmt.Sprintf("format: %q is not one of terminal, json; keeping %q", *f.Format, cfg.Format))
		}
	}

	if f.GraphPath != nil && *f.GraphPath != "" {
		p := *f.GraphPath
		if !filepath.IsAbs(p) {
			p = filepath.Join(configDir, p)
		}
		cfg.GraphPath = p
	}

	if len(f.Ignore) > 0 {
		cfg.Ignore = f.Ignore
	}

	if f.Risk != nil {
		// A threshold of 0 would classify every affected function as HIGH, which
		// is a configuration nobody wants and every typo produces. 1 is the
		// strictest useful value: any production caller at all.
		atLeast("risk.high_callers", f.Risk.HighCallers, 1, &cfg.Risk.HighCallers)
		atLeast("risk.high_exported_callers", f.Risk.HighExportedCallers, 1, &cfg.Risk.HighExportedCallers)
		atLeast("risk.medium_callers", f.Risk.MediumCallers, 1, &cfg.Risk.MediumCallers)
	}

	return warn
}

func joinWarnings(w []string) error {
	if len(w) == 0 {
		return nil
	}
	return fmt.Errorf("ignored invalid config value(s): %s", strings.Join(w, "; "))
}

// findConfigFile walks up from dir looking for .tracescope.yaml or .tracescope.yml.
func findConfigFile(dir string) string {
	for {
		for _, name := range []string{".tracescope.yaml", ".tracescope.yml"} {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
