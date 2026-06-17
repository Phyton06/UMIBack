.PHONY: build run test tidy migrate-up migrate-down

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

# Ejecuta migraciones pendientes (requiere DB_URL)
migrate-up:
	go run ./cmd/server

# No implementado: migraciones down requieren comando separado
migrate-down:
	@echo "Usa 'migrate-down' directamente con golang-migrate CLI"
	@echo "Ejemplo: migrate -path internal/db/migrations -database \"$$DB_URL\" down 1"
