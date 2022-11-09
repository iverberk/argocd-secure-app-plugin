package main

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// defines the options with which to kustomize build
func buildKustomizeFlags(opts *krusty.Options) *krusty.Options {
	c := types.EnabledPluginConfig(types.BploUseStaticallyLinked)
	opts.PluginConfig = c
	opts.DoLegacyResourceSort = true

	return opts
}

// build runs kustomize with the options defined in buildKustomizeFlags
func build(source string) ([]byte, error) {
	fSys := filesys.MakeFsOnDisk()

	k := krusty.MakeKustomizer(
		buildKustomizeFlags(krusty.MakeDefaultOptions()),
	)
	m, err := k.Run(fSys, source)
	if err != nil {
		return nil, err
	}
	yml, err := m.AsYaml()
	if err != nil {
		return nil, err
	}
	return yml, nil
}

// kustomize runs `kustomize build` on the kustomize directory,
// contrary to helm and manifests, kustomize does NOT decrypt sops due to
// kustomize reordering data fields, making sops mac-check fail:
// https://github.com/mozilla/sops/issues/833
// use a generator instead
func kustomize(source string) ([]byte, error) {
	if _, err := os.Stat(source + "/kustomization.yaml"); err != nil {
		log.Warn().Err(err).Str("source", source).Msg("unable to load kustomization.yaml, assuming this is not a Kustomize dir")
		return nil, nil
	}

	out, err := build("./" + source)
	if err != nil {
		return nil, err
	}

	// Check if we need to transform the Kustomize output.
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
