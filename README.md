# gophprofile

## Demo

List all available tasks:
```
task -l
```

### Docker Compose

Start containers:
```
task compose-up
```

`curl` commands:
- `GET /health`:

    curl "localhost:8080/health"
- `POST /api/v1/avatars`:

    curl -X POST "localhost:8080/api/v1/avatars" -H "X-User-ID: __{string}__" -F "file=@__{/path/to/image.jpeg}__"
- `GET /api/v1/avatars/{id}`:

    curl "localhost:8080/api/v1/avatars/__{uuid}__" --output __{image.jpg}__
- `GET /api/v1/avatars/{id}/metadata`:

    curl "localhost:8080/api/v1/avatars/__{uuid}__/metadata"
- `DELETE /api/v1/avatars/{id}`:

    curl -X DELETE "localhost:8080/api/v1/avatars/__{uuid}__" -H "X-User-ID: __{string}__"

Web UI:
- `MinIO`: 

    http://localhost:9001, user:password
- `RabbitMQ`: 

    http://localhost:15672, user:password
- `Jaeger`: 

    http://localhost:16686
- `Prometheus`: 

    http://localhost:9090
- `Grafana`: 

    http://localhost:3000, user:password

Stop containers:
```
task compose-down
```

### Kubernetes

Build Docker images:
```
task build-images
```

Before deploying, create `deploy/k8s/secret.yml` from the example and replace the values with your own.

Deploy gophprofile:
```
task kube-up
```

Remove gophprofile:
```
task kube-down
```

`curl` commands:
- `GET /health`:

    curl -H "Host: gophprofile.localhost" "localhost/health"
- `POST /api/v1/avatars`:

    curl -X POST -H "Host: gophprofile.localhost" "localhost/api/v1/avatars" -H "X-User-ID: __{string}__" -F "file=@__{/path/to/image.jpeg}__"
- `GET /api/v1/avatars/{id}`:

    curl -H "Host: gophprofile.localhost" "localhost/api/v1/avatars/__{uuid}__" --output __{image.jpg}__
- `GET /api/v1/avatars/{id}/metadata`:

    curl -H "Host: gophprofile.localhost" "localhost/api/v1/avatars/__{uuid}__/metadata"
- `DELETE /api/v1/avatars/{id}`:

    curl -X DELETE -H "Host: gophprofile.localhost" "localhost/api/v1/avatars/__{uuid}__" -H "X-User-ID: __{string}__"
