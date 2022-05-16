CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build main.go
docker run -v $(pwd)/:/app alpine .//app/main
