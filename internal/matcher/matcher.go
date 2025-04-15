package matcher

import (
	"strings"

	"github.com/matthewmueller/glob"
)

// Compile into a matcher function
func Compile(includes []string, excludes []string) (func(path string) bool, error) {
	// Create include matcher
	include, err := includer(includes...)
	if err != nil {
		return nil, err
	}
	// Create exclude matcher
	exclude, err := excluder(excludes...)
	if err != nil {
		return nil, err
	}
	// Return the final matcher function
	return func(path string) bool {
		if exclude(path) {
			return false
		}
		return include(path)
	}, nil
}

func isGlob(pattern string) bool {
	return glob.Base(pattern) != pattern
}

func compilePattern(pattern string) (func(path string) bool, error) {
	if isGlob(pattern) {
		matcher, err := glob.Compile(pattern)
		if err != nil {
			return nil, err
		}
		return matcher.Match, nil
	}
	return func(path string) bool {
		return strings.Contains(path, pattern)
	}, nil
}

func includer(includes ...string) (func(path string) bool, error) {
	matchers := make([]func(path string) bool, len(includes))
	for i, include := range includes {
		matcher, err := compilePattern(include)
		if err != nil {
			return nil, err
		}
		matchers[i] = matcher
	}
	return func(path string) bool {
		if len(matchers) == 0 {
			return true
		}
		// Include if any of the matchers match
		for _, matcher := range matchers {
			if matcher(path) {
				return true
			}
		}
		return false
	}, nil
}

func excluder(excludes ...string) (func(path string) bool, error) {
	matchers := make([]func(path string) bool, len(excludes))
	for i, exclude := range excludes {
		matcher, err := compilePattern(exclude)
		if err != nil {
			return nil, err
		}
		matchers[i] = matcher
	}
	return func(path string) bool {
		if len(matchers) == 0 {
			return false
		}
		// Exclude if any of the matchers match
		for _, matcher := range matchers {
			if matcher(path) {
				return true
			}
		}
		return false
	}, nil
}
