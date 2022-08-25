package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func createPrivateHelmRepos(chart *chart.Chart) {
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

// loadValues retrieves values from the Helm source directory. It loads a values.yaml file
// from the root of the source and it scans a 'values' directory for any additional values.
// Any values found in the 'values' directory will act as value overrides.
func loadValues(source string) map[string]interface{} {
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

	return values
}

// getDependencies downloads all the Helm chart dependencies.
func getDependencies(source string) {
	// Download chart dependencies
	out, err := exec.Command("helm", "dependency", "update", source).CombinedOutput()
	if err != nil {
		log.Fatal().Str("error output", string(out)).Err(err).Msg("Helm dependency update command failed")
	}
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
		createPrivateHelmRepos(chart)
	}

	// Load the Helm chart values
	values := loadValues(source)

	// Download any required dependencies for the Helm chart.
	getDependencies(source)

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

	out := stdout.Bytes()

	// Check if we need to transform the Helm output.
	f := filepath.Join(source, "transform.jq")
	query, err := os.ReadFile(f)
	if err == nil {
		out = transform(out, string(query))
	} else {
		log.Warn().Err(err).Msg("Unable to load JQ transform file, skipping transformation")
	}

	// Return the generated manifests
	return out, nil
}
