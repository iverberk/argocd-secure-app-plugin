package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
)

// buildSources builds a map of unique directories that contain at least one YAML file.
// These directories are considered sources that need to be processed for resource generation.
func buildSources(rootDir string) []string {
	log.Info().Str("root directory", rootDir).Msg("Scanning for sources")

	sources := make(map[string]bool)

	fsys := os.DirFS(rootDir)
	fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {

		source := filepath.Join(rootDir, p)

		if filepath.Ext(source) == ".yaml" {
			log.Info().Str("source", source).Msg("Located a potential source path")

			// Mark this path as a source
			sources[filepath.Dir(source)] = true

			// We are only interested in the existence of a single YAML file.
			// If we find one, treat it as a source and skip the rest of the
			// directory contents. Because filesystem walking happens in
			// lexicographical order, it might happen that subdirectories have
			// already been added as source. We will prune these later as we
			// don't allow nested sources.
			return fs.SkipDir
		}
		return nil
	})

	// Keep only top-level source paths. Deleting items from sources while
	// iterating is safe because deleted items will simply be skipped.
	for s1 := range sources {
		for s2 := range sources {
			if s1 != s2 && strings.HasPrefix(s2, s1+"/") {
				log.Info().Str("root source", s1).Str("sub-source", s2).Msg("Pruning subpath source from root source")
				delete(sources, s2)
			}
		}
	}

	// Make the source iteration deterministic by sorting the paths.
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	return paths
}

// generateManifests processes each source to generate manifests. This can be done through
// a combination of plain manifests, Helm chart, Kustomize and potential transformations.
func generateManifests(rootDir string) string {

	// Build a list of source directories
	sources := buildSources(rootDir)

	var result []byte
	for _, source := range sources {
		log.Info().Str("source", source).Msg("Processing source")

		output, err := helm(source)
		if output != nil && err == nil {
			result = append(result, output...)
			continue
		}

		output, err = kustomize(source)
		if output != nil && err == nil {
			result = append(result, output...)
			continue
		}

		output, err = manifests(source)
		if err != nil {
			log.Error().Err(err).Msg("Unable to parse source for manifests")
		}
		result = append(result, output...)
	}

	return string(result)
}
