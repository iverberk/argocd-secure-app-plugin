package main

import (
	"github.com/rs/zerolog/log"
	sopsDecrypt "go.mozilla.org/sops/v3/decrypt"
	"gopkg.in/yaml.v2"
)

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
