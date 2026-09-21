# gophprofile

## Demo

List all available tasks:
```
task -l
```

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

Stop containers:
```
task compose-down
```
