package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	sopsDecrypt "go.mozilla.org/sops/v3/decrypt"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart/loader"
)

const YAML_DELIMITER = "---\n"

// decrypt takes a raw YAML document and checks if a top-level `sops` key
// exists. If it does, it will decrypt the data in-place.
func decrypt(data []byte) []byte {
	content := make(map[string]interface{})
	err := yaml.Unmarshal(data, &content)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to parse YAML data")
	}

	if _, ok := content["sops"]; ok {
		var err error
		data, err = sopsDecrypt.Data(data, "yaml")
		if err != nil {
			log.Fatal().Str("data", string(data)).Err(err).Msg("Unable to decrypt data with SOPS")
		}
	}

	return data
}

// mergeMaps merges two maps. In case of equal keys, both in level and name, the value
// from the second map takes precedence.
func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

// helm tries to load a Helm chart and run the template command,
// after potentially decrypting the values with SOPS.
func helm(source string) ([]byte, error) {

	// Load the Chart metadata.
	chart, err := loader.Load(source)
	if err != nil {
		log.Warn().Err(err).Str("source", source).Msg("Unable to load Helm chart data, assuming this is not a Helm chart")
		return nil, nil
	}

	// Check if we are pulling from a private Helm repository. If so, pre-add all the repositories
	// with username and password. Also set a custom CA bundle if it is provided.
	if _, ok := os.LookupEnv("ARGOCD_ENV_HELM_PRIVATE"); ok {

		// Build credentials arguments
		var credArgs []string
		username := os.Getenv("ARGCDO_ENV_HELM_USERNAME")
		password := os.Getenv("ARGCDO_ENV_HELM_PASSWORD")
		if username != "" && password != "" {
			credArgs = append(credArgs, "--username", username, "--password", password)
		}

		// Build CA bundle arguments
		var caArgs []string
		caFile := os.Getenv("ARGOCD_ENV_HELM_CA_FILE")
		if caFile != "" {
			caArgs = append(caArgs, "--ca-file", caFile)
		}

		for _, dependency := range chart.Metadata.Dependencies {
			log.Info().Str("Repository", dependency.Repository).Msg("Adding new Helm repo")

			var args = []string{"repo", "add", dependency.Name, dependency.Repository, "--force-update"}

			args = append(args, credArgs...)
			args = append(args, caArgs...)

			out, err := exec.Command("helm", args...).CombinedOutput()
			if err != nil {
				log.Fatal().Str("error output", string(out)).Err(err).Msg("Helm repo add command failed")
			}
		}
	}

	// Gather values files. We check for a `values.yaml` file in the root directory.
	// All other values files, encrypted or not, are expected to be in a `values` subdirectory.
	var values map[string]interface{}

	f := filepath.Join(source, "values.yaml")
	data, err := os.ReadFile(f)
	if err == nil {
		// Decode the values file to a map
		if err := yaml.Unmarshal(decrypt(data), &values); err != nil {
			log.Fatal().Str("file", f).Err(err).Msg("Unable to YAML decode values file")
		}
	} else {
		log.Warn().Err(err).Msg("Unable to load default values file, checking for other values")
	}

	addVfs, err := os.ReadDir(filepath.Join(source, "values"))
	if err != nil {
		log.Warn().Err(err).Msg("Unable to read the values directory, assuming no additional values files exist")
	} else {
		for _, vf := range addVfs {
			if vf.IsDir() {
				continue
			}

			f := filepath.Join(source, "values", vf.Name())
			data, err := os.ReadFile(f)
			if err == nil {
				// Decode the values file to a map

				var overrides map[string]interface{}
				if err := yaml.Unmarshal(decrypt(data), &overrides); err != nil {
					log.Fatal().Str("file", f).Err(err).Msg("Unable to YAML decode values file")
				}

				values = mergeMaps(values, overrides)
			} else {
				log.Fatal().Str("file", f).Err(err).Msg("Unable to read values file")
			}
		}
	}

	// Download chart dependencies
	out, err := exec.Command("helm", "dependency", "update", source).CombinedOutput()
	if err != nil {
		log.Fatal().Str("error output", string(out)).Err(err).Msg("Helm dependency update command failed")
	}

	// Execute Helm templating
	cmd := exec.Command("helm", "template", "-n", os.Getenv(`ARGOCD_APP_NAMESPACE`), os.Getenv(`ARGOCD_APP_NAME`), source, "-f", "-")

	// Marshall the values map to an array of bytes
	b, err := yaml.Marshal(values)
	if err != nil {
		log.Fatal().Err(err).Msg("Could not marshall the values map to bytes")
	}

	// Feed the values via stdin
	cmd.Stdin = bytes.NewReader(b)
	var stderr = bytes.Buffer{}
	var stdout = bytes.Buffer{}
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		log.Fatal().Str("error output", stderr.String()).Err(err).Msg("Helm template command failed")
	}

	// Return the generated manifests
	return stdout.Bytes(), nil
}

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

// kustomize applies the Kustomize configuration to the resources
// that have been generated by helm and plain manifests.
//
// IMPORTANT: As kustomize does not allow input from stdin, we will have to write the resources
// temporarily to disk, including any *decrypted* secrets. They will be cleaned up immediately
// afterwards but if the plugin crashes it might leave decrypted secrets hanging around.
func kustomize(resources []byte) ([]byte, error) {
	return nil, nil
}

// generate builds a map of unique directories that contain at least one YAML file.
// These directories are considered sources that need to be processed for resource generation.
func generate(rootDir string) string {
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

	var result []byte
	for _, path := range paths {
		log.Info().Str("source", path).Msg("Processing source")

		output, err := helm(path)
		if output != nil && err == nil {
			result = append(result, output...)
			continue
		}

		output, err = manifests(path)
		if err != nil {
			log.Error().Err(err).Msg("Unable to parse source for manifests")
		}
		result = append(result, output...)
	}

	return string(result)
}

func main() {
	// By default only log errors so that we don't clutter the output
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)

	v := flag.Bool("v", false, "Verbose logging (info level, for debugging only)")
	flag.Parse()

	if *v {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to determine current directory")
	}

	// Argo CD adds an ARGOCD_ENV_ prefix to all application-defined environment variables.
	// Strip that prefix so that SOPS can find its decryption key based on SOPS_AGE_KEY_FILE.
	if value, ok := os.LookupEnv("ARGOCD_ENV_SOPS_AGE_KEY_FILE"); ok {
		os.Setenv("SOPS_AGE_KEY_FILE", strings.TrimPrefix(value, "ARGOCD_ENV_"))
	}

	fmt.Printf("%s", generate(rootDir))
}
