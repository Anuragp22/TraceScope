package config

import (
	"os"
	"path/filepath"

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

	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return cfg, configPath, err
	}

	// Merge file values into defaults (only override non-zero values)
	if fileCfg.MaxDepth > 0 {
		cfg.MaxDepth = fileCfg.MaxDepth
	}
	if fileCfg.Format != "" {
		cfg.Format = fileCfg.Format
	}
	if fileCfg.TopN > 0 {
		cfg.TopN = fileCfg.TopN
	}
	if fileCfg.GraphPath != "" {
		cfg.GraphPath = fileCfg.GraphPath
		// Resolve relative paths against config file location
		if !filepath.IsAbs(cfg.GraphPath) {
			cfg.GraphPath = filepath.Join(filepath.Dir(configPath), cfg.GraphPath)
		}
	}
	if len(fileCfg.Ignore) > 0 {
		cfg.Ignore = fileCfg.Ignore
	}
	if fileCfg.Risk.HighCallers > 0 {
		cfg.Risk.HighCallers = fileCfg.Risk.HighCallers
	}
	if fileCfg.Risk.HighExportedCallers > 0 {
		cfg.Risk.HighExportedCallers = fileCfg.Risk.HighExportedCallers
	}
	if fileCfg.Risk.MediumCallers > 0 {
		cfg.Risk.MediumCallers = fileCfg.Risk.MediumCallers
	}

	return cfg, configPath, nil
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
