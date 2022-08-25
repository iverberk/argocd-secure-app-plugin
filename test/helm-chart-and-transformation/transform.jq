del(. | select(.kind == "Deployment" and .metadata.name == "test-hello-world")) | select(. != null)
