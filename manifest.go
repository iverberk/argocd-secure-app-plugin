package main

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// manifests checks a directory for plain Kubernetes YAML resource files.
// Decryption happens automatically if a top-level `sops` key exists.
func manifests(path string) ([]byte, error) {
	var manifests []byte

	files, err := os.ReadDir(path)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to read the manifests directory")
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		manifest, err := os.ReadFile(filepath.Join(path, f.Name()))
		if err == nil {
			if len(manifest) == 0 {
				log.Warn().Str("file", f.Name()).Err(err).Msg("Skipping empty manifest file")
				continue
			}
			manifest = append([]byte(YAML_DELIMITER), decrypt(manifest)...)
			manifests = append(manifests, manifest...)
			manifests = append(manifests, []byte("\n")...)
		} else {
			log.Fatal().Str("file", f.Name()).Err(err).Msg("Unable to read manifest file")
		}
	}

	return manifests, nil
}
