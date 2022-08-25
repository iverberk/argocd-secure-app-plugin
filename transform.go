package main

import (
	"bytes"
	"io"

	"github.com/itchyny/gojq"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

// transform takes a raw multi-document YAML and transforms it using a jq query.
func transform(source []byte, q string) []byte {

	var out bytes.Buffer

	query, err := gojq.Parse(q)
	if err != nil {
		log.Fatal().Str("query string", q).Err(err).Msg("Unable to parse jq query string")
	}

	code, err := gojq.Compile(query)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to compile jq query string")
	}

	// Decode all documents in the source.
	dec := yaml.NewDecoder(bytes.NewReader(source))
	for {

		var v interface{}
		if err := dec.Decode(&v); err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal().Err(err).Msg("Error occurred during YAML decoding")
		}

		v = normalizeYAML(v)
		transformDoc(v, &out, code)
	}

	return out.Bytes()
}

// transform applies a JQ transformation to a YAML document.
func transformDoc(v interface{}, out *bytes.Buffer, code *gojq.Code) {

	iter := code.Run(v) // TODO: change to RunWithContext for guaranteed termination
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			log.Fatal().Err(err).Msg("Error occurred during jq query run")
		}

		out.Write([]byte(YAML_DELIMITER))
		enc := yaml.NewEncoder(out)
		if err := enc.Encode(v); err != nil {
			log.Fatal().Err(err).Msg("Error occurred during YAML encoding of transformed data")
		}
		enc.Close()
	}
}
