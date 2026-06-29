# UMI — Guía de Deploy en AWS

## Quick path

```bash
# 1. Requisitos previos
brew install terraform awscli           # macOS
# o: choco install terraform awscli      # Windows

# 2. Configurar credenciales AWS
aws configure

# 3. Crear bucket S3 para Terraform state (una vez)
aws s3 mb s3://umi-terraform-state --region us-east-1

# 4. Crear zona Route 53 (si no existe)
aws route53 create-hosted-zone --name umi.app --caller-reference $(date +%s)

# 5. Desplegar infraestructura
cd terraform
terraform init
terraform plan
terraform apply

# 6. Construir y desplegar el backend
aws ecr get-login-password | docker login --username AWS --password-stdin $(terraform output -raw ecr_repository)
docker build -t umi-backend ..
docker tag umi-backend:latest $(terraform output -raw ecr_repository):latest
docker push $(terraform output -raw ecr_repository):latest
aws ecs update-service --cluster umi-cluster --service umi-backend --force-new-deployment

# 7. Desplegar panel admin
cd ../umi_admin
flutter build web --dart-define=API_BASE_URL=$(cd ../terraform && terraform output -raw api_url)
aws s3 sync build/web/ s3://umi-admin-production/ --delete
```

## Arquitectura de despliegue

```
Internet
  │
  ├─ api.umi.app (HTTPS) ──► ALB ──► ECS Fargate (Docker) ──► RDS PostgreSQL + PostGIS
  │                              │
  │                              └── CloudWatch Logs + Métricas
  │
  └─ admin.umi.app (HTTPS) ──► CloudFront ──► S3 (static Flutter Web)
```

## Variables de entorno

| Variable | Origen | Descripción |
|----------|--------|-------------|
| `DB_URL` | Secrets Manager | Conexión PostgreSQL (formato `postgres://...`) |
| `JWT_SECRET` | Secrets Manager | Clave HS256 para firmar JWT |
| `PORT` | ECS task definition | Puerto HTTP (8080) |
| `LOG_LEVEL` | ECS task definition | Nivel de log (info) |

## Pipeline CI/CD

| Pipeline | Trigger | Acción |
|----------|---------|--------|
| `deploy-backend.yml` | Push a `main` con cambios en `*.go` o `Dockerfile` | Build Docker → push ECR → force deploy ECS |
| `deploy-admin.yml` | Push a `main` con cambios en `umi_admin/` | Build Flutter Web → sync S3 → invalidar CloudFront |
| `keepalive.yml` | Lunes y viernes 8:00 UTC | `curl /health` para mantener caliente Supabase |

## Rollback

```bash
# Backend: desplegar commit anterior
git revert HEAD && git push

# Infraestructura: restaurar state anterior
cd terraform && terraform state pull > backup.tfstate
# Si algo falla: terraform state push backup.tfstate

# Admin: restaurar build anterior desde S3 versioning
aws s3api list-object-versions --bucket umi-admin-production --prefix index.html
aws s3api copy-object ... --version-id <previous-version>
```
