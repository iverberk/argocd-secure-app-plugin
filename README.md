# ArgoCD Secure App Plugin

This repository contains a simple Argo CD plugin that serves the following purposes:

- Allow multiple sources (Helm charts, plan manifests, Kustomize (TODO)) to generate application resources
- Automatically decrypt SOPS encrypted files while processing
- Use standard input to pass decrypted resources. Never write decrypted files to disk unless absolutely necessary.

## Credits

This plugins is largely inspired by the [argocd-lovely-plugin](https://github.com/crumbhole/argocd-lovely-plugin). The only reason this plugin exists, is because I needed to integrate SOPS into the worklow in a simple and secure way, meaning no decrypted written files to disk. Also, I didn't quite need all the features that the lovely plugin provides. I do recommend that you check it out to see if it fits your needs.

## How it works

The plugin scans the current directory for any subdirectories that contain YAML files. Each subdirectory it finds is considered a potential source. When the scan is completed, all the subdirectory paths are inspected and pruned to make sure that sources are not nested. A recommended structure for sources is:

```bash
app/helm-app-1        # A Helm chart to deploy app 1 (contains Chart.yaml and potentially a values.yaml)
app/helm-app-2        # A Helm chart to deploy app 2 (contains Chart.yaml and potentially a values.yaml)
app/helm-app-2/values # Additional Helm values for app 2
app/manifests         # Plain Kubernetes manifests
app/secrets           # Encrypted Kubernetes manifests
```

### SOPS Decryption

The plugin scans each YAML file for a top-level key called 'sops'. If it finds this key, it will automatically decrypt the file with SOPS.

### Helm

Each source directory is checked for the existence of a `Chart.yaml` file. If the chart file exists, the source is treated as a Helm chart. By default, the `values.yaml` file in the same directory (if it exists) is loaded and automatically decrypted. Additional (encrypted) Helm values can be placed in a subdirectory called `values`. They will be added to the Helm command in _lexicographic_ order, keep this in mind if you want to override values.

### Manifests

You can create subdirectories with (encrypted) plain YAML manifests. These will be decrypted if necessary and fed to Kubernetes as-is.

## Running the plugin

### Locally

Build the plugin and make sure that the binary is somewhere in your path. Move to the directory that you would like to test and just run the binary. For example, if your ArgoCD app lives in `apps/dex` then run `cd apps/dex && argocd-secure-app-plugin`. This should provide you with an output of resources, ready to be fed.

**IMPORTANT**: if you use Helm charts, you need to set the `ARGOCD_APP_NAME` environment variable so that Helm correctly sets the metadata on resources.

### Within ArgoCD

TODO: Create a plugin docker image and add it as an additional container to the ArgoCD depoyment.

## Development and Testing

You can develop this plugin with Go 1.18. Tests can be run with `go test ./...`. The format of the tests should be self-explanatory if you look at the examples in the `test` directory.
