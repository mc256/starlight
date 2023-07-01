# Docker Compose Dev environment

The `docker-compose-example.yaml` file is an example for launching a local development environment.
You could edit it and save it as `docker-compose.yaml` file

| service needed for developing | starlight-proxy | db                 | registry          |             |
| ----------------------------- | --------------- | ------------------ | ----------------- | ----------- |
| ctr-starlight                 | ✅               | ✅                  | ✅                 | plus daemon |
| starlight-daemon              | ✅               | ✅                  | ✅                 |             |
| starlight-proxy               |                 | ✅                  | ✅                 |             |
| ----------------              | --------------- | ------------------ | ----------------- | ----        |
