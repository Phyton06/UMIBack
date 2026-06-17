# UMI Backend

Backend de la plataforma de transporte UMI. Servicio en Go con comunicación en tiempo real vía WebSockets, geolocalización con PostGIS y caché distribuida con Redis.

## Stack

- **Lenguaje:** Go
- **Comunicación en tiempo real:** WebSockets
- **Base de datos:** PostgreSQL + PostGIS
- **Caché:** Redis
- **Contenedores:** Podman / Docker

## Estructura del proyecto

```
.
├── cmd/              # Puntos de entrada de la aplicación
├── internal/         # Código interno (no exportable)
│   ├── api/         # Handlers HTTP y WebSocket
│   ├── db/          # Consultas y migraciones
│   ├── model/       # Modelos de dominio
│   └── engine/      # Lógica de negocio y máquina de estados
├── pkg/             # Librerías compartibles
└── docker/          # Archivos de contenedor
```

*Estructura sujeta a cambios durante el desarrollo inicial.*

## Requisitos

- Go 1.22+
- PostgreSQL 16+ con PostGIS
- Redis 7+

## Desarrollo

```bash
# Inicializar módulo
go mod init github.com/Phyton06/UMIBack

# Ejecutar pruebas
go test ./...
```

## Licencia

Uso interno — proyecto privado.
