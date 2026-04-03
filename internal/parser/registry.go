package parser

import (
	"crypto/sha256"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/rs/zerolog/log"
)

// Registry maps languages to their parsers and orchestrates parsing.
type Registry struct {
	parsers map[Language]LanguageParser
}

// NewRegistry creates a registry with all available parsers.
func NewRegistry() *Registry {
	r := &Registry{
		parsers: make(map[Language]LanguageParser),
	}
	r.register(NewGoParser())
	r.register(NewJavaScriptParser())
	r.register(NewTypeScriptParser())
	r.register(NewPythonParser())
	return r
}

func (r *Registry) register(p LanguageParser) {
	r.parsers[p.Language()] = p
}

// ParseFiles parses all files concurrently using a worker pool.
func (r *Registry) ParseFiles(filesByLang map[Language][]string) ([]*FileResult, []error) {
	type work struct {
		path string
		lang Language
	}

	var jobs []work
	for lang, files := range filesByLang {
		if _, ok := r.parsers[lang]; !ok {
			continue
		}
		for _, f := range files {
			jobs = append(jobs, work{path: f, lang: lang})
		}
	}

	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if len(jobs) < workers {
		workers = len(jobs)
	}
	if workers == 0 {
		return nil, nil
	}

	var (
		mu      sync.Mutex
		results []*FileResult
		errs    []error
		wg      sync.WaitGroup
	)

	jobCh := make(chan work, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := range jobCh {
				p := r.parsers[j.lang]
				// Check file size — skip files > 10MB
				stat, err := os.Stat(j.path)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("stat %s: %w", j.path, err))
					mu.Unlock()
					continue
				}
				if stat.Size() > 10*1024*1024 {
					log.Debug().Str("file", j.path).Int64("size", stat.Size()).Msg("skipping large file")
					continue
				}

				source, err := os.ReadFile(j.path)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("reading %s: %w", j.path, err))
					mu.Unlock()
					continue
				}
				result, err := p.Parse(j.path, source)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("parsing %s: %w", j.path, err))
					mu.Unlock()
					continue
				}
				// Compute content hash
				h := sha256.Sum256(source)
				result.ContentHash = fmt.Sprintf("%x", h)
				log.Debug().Str("file", j.path).
					Int("functions", len(result.Functions)).
					Int("calls", len(result.Calls)).
					Int("imports", len(result.Imports)).
					Msg("parsed")
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	enrichTypeScriptSemantics(results)
	return results, errs
}
