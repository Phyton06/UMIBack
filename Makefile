.PHONY: build run test tidy

# Compila el binario del servidor
build:
	go build -o bin/server ./cmd/server

# Ejecuta el servidor en desarrollo
run:
	go run ./cmd/server

# Ejecuta todas las pruebas
test:
	go test ./... -v -count=1

# Limpia las dependencias del módulo
tidy:
	go mod tidy
