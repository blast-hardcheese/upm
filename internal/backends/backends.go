// Package backends contains the language-specific UPM backends, and
// logic for selecting amongst them.
package backends

import (
	"context"
	"strings"

	"github.com/replit/upm/internal/api"
	"github.com/replit/upm/internal/backends/dart"
	"github.com/replit/upm/internal/backends/dotnet"
	"github.com/replit/upm/internal/backends/elisp"
	"github.com/replit/upm/internal/backends/java"
	"github.com/replit/upm/internal/backends/nodejs"
	"github.com/replit/upm/internal/backends/php"
	"github.com/replit/upm/internal/backends/python"
	"github.com/replit/upm/internal/backends/rlang"
	"github.com/replit/upm/internal/backends/ruby"
	"github.com/replit/upm/internal/backends/rust"
	"github.com/replit/upm/internal/util"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// languageBackends is a slice of language backends which may be used
// from the command line.
//
// If more than one backend might match the same project, then one
// that comes first in this list will be used.
var languageBackends = []api.LanguageBackend{
	python.PythonPoetryBackend,
	python.PythonPipBackend,
	nodejs.BunBackend,
	nodejs.NodejsNPMBackend,
	nodejs.NodejsPNPMBackend,
	nodejs.NodejsYarnBackend,
	ruby.RubyBackend,
	elisp.ElispBackend,
	dart.DartPubBackend,
	java.JavaBackend,
	rlang.RlangBackend,
	dotnet.DotNetBackend,
	rust.RustBackend,
	php.PhpComposerBackend,
}

// matchesLanguage checks if a language backend matches a value for
// the --lang argument. For example, the python-python3-poetry backend
// will match --lang=python-poetry and --lang=python3 but not
// --lang=python2. This is used to filter the available language
// backends before trying to guess which one should be used.
func matchesLanguage(b api.LanguageBackend, language string) bool {
	bParts := map[string]bool{}
	for _, bPart := range strings.Split(b.Name, "-") {
		bParts[bPart] = true
	}
	checkAlias := false
	for _, lPart := range strings.Split(language, "-") {
		if !bParts[lPart] {
			checkAlias = true
			break
		}
	}
	if checkAlias {
		bParts = map[string]bool{}
		for _, bPart := range strings.Split(b.Alias, "-") {
			bParts[bPart] = true
		}
		for _, lPart := range strings.Split(language, "-") {
			if !bParts[lPart] {
				return false
			}
		}
	}
	return true
}

// GetBackend returns the language backend for a given --lang argument
// value. If none is applicable, it exits the process.
func GetBackend(ctx context.Context, language string) api.LanguageBackend {
	//nolint:ineffassign,wastedassign,staticcheck
	span, ctx := tracer.StartSpanFromContext(ctx, "GetBackend")
	defer span.Finish()
	backends := languageBackends
	if language != "" {
		filteredBackends := []api.LanguageBackend{}
		for _, b := range backends {
			if matchesLanguage(b, language) {
				filteredBackends = append(filteredBackends, b)
			}
		}
		switch len(filteredBackends) {
		case 0:
			util.Die("no such language: %s", language)
		case 1:
			return filteredBackends[0]
		default:
			backends = filteredBackends
		}

	}
	for _, b := range backends {
		if util.Exists(b.Specfile) &&
			util.Exists(b.Lockfile) {
			return b
		}
	}
	for _, b := range backends {
		if util.Exists(b.Specfile) ||
			util.Exists(b.Lockfile) {
			return b
		}
	}
	for _, b := range backends {
		for _, p := range b.FilenamePatterns {
			if util.PatternExists(p) {
				return b
			}
		}
	}
	if language == "" {
		util.Die("could not autodetect a language for your project")
	}
	return backends[0]
}

type BackendInfo struct {
	Name      string
	Available bool
}

// GetBackendNames returns a slice of the canonical names (e.g.
// python-python3-poetry, not just python3) for all the backends
// listed in languageBackends.
func GetBackendNames() []BackendInfo {
	var backendNames []BackendInfo
	for _, b := range languageBackends {
		backendNames = append(backendNames, BackendInfo{Name: b.Name, Available: b.IsAvailable()})
	}
	return backendNames
}

// SetupAll panics if any registered language backend does not
// implement its mandatory fields. It also assigns defaults for all
// registered language backends.
func SetupAll() {
	for i := range languageBackends {
		// Make sure that the Setup function can make changes
		// to the struct.
		(&languageBackends[i]).Setup()
	}
}
