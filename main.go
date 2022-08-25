package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// By default only log errors so that we don't clutter the output.
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

	// Print all the manifests that have been generated to standard output.
	fmt.Printf("%s", generateManifests(rootDir))
}
