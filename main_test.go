// CREDITS: This is mostly taken from: https://github.com/crumbhole/argocd-lovely-plugin/blob/main/main_test.go
package main

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"testing"
)

const (
	testsPath = "test/"
)

func setupEnv(path string) (map[string]string, error) {
	var envValues map[string]string
	envFile := path + "/env.txt"
	_, err := os.Stat(envFile)
	if !errors.Is(err, os.ErrNotExist) {
		envText, err := ioutil.ReadFile(envFile)
		if err != nil {
			return envValues, err
		}
		if err := yaml.Unmarshal(envText, &envValues); err != nil {
			return envValues, err
		}
		for k, v := range envValues {
			os.Setenv(k, v)
		}
	}
	return envValues, nil
}

func cleanupEnv(env map[string]string) {
	for k := range env {
		os.Unsetenv(k)
	}
}

func checkDir(path string) error {
	env, err := setupEnv(path)
	defer cleanupEnv(env)
	if err != nil {
		return err
	}

	resources := generate(path)

	expected, err := ioutil.ReadFile(path + "/expected.txt")
	if err != nil {
		return err
	}

	if resources != string(expected) {
		ioutil.WriteFile(path+"/got.txt", []byte(resources), 0444)
		return fmt.Errorf("Expected >\n%s\n< and got >\n%s\n<", expected, resources)
	}
	return nil
}

// Finds directories under ./test and evaluates all the .yaml/.ymls
func TestDirectories(t *testing.T) {
	os.Setenv(`ARGOCD_APP_NAME`, `test`)
	os.Setenv(`ARGOCD_APP_NAMESPACE`, `testnamespace`)
	os.Setenv(`SOPS_AGE_KEY`, `AGE-SECRET-KEY-1NY09NVG886V8NHJSYT62DMTLVFG7LWRLE8VPMTA6UJE05W2X20US2Y74X3`)
	dirs, err := ioutil.ReadDir(testsPath)
	if err != nil {
		t.Error(err)
	}

	for _, d := range dirs {
		if d.IsDir() {
			t.Run(d.Name(), func(t *testing.T) {
				err := checkDir(testsPath + d.Name())
				if err != nil {
					t.Error(err)
				}
			})
		}
	}
}
