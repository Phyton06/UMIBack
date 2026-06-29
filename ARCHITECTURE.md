# UMI — Arquitectura de Red (AWS)

## Diagrama de red

```
                         Internet
                            │
                    ┌───────┴───────┐
                    │   Route 53     │
                    │ api.umi.app    │
                    │ admin.umi.app  │
                    └───────┬───────┘
                            │
              ┌─────────────┴─────────────┐
              │                           │
      ┌───────┴───────┐           ┌───────┴───────┐
      │   CloudFront   │           │      ALB       │
      │  (admin web)   │           │  (HTTPS:443)   │
      └───────┬───────┘           └───────┬───────┘
              │                           │
      ┌───────┴───────┐           ┌───────┴───────┐
      │   S3 bucket    │           │  ECS Fargate   │
      │ (flutter web)  │           │  (Docker:8080) │
      └───────────────┘           └───────┬───────┘
                                          │
                          ┌───────────────┼───────────────┐
                          │               │               │
                  ┌───────┴───────┐ ┌─────┴─────┐ ┌───────┴───────┐
                  │ Secrets Mgr   │ │ CloudWatch │ │     RDS       │
                  │ DB_URL, JWT   │ │  Logs      │ │ PostgreSQL 17 │
                  └───────────────┘ └───────────┘ │   + PostGIS   │
                                                  └───────────────┘
```

## VPC — Subredes

```
VPC: 10.0.0.0/16
├── Subred pública 1 (AZ a):  10.0.0.0/24   ── Internet Gateway
├── Subred pública 2 (AZ b):  10.0.1.0/24   ── Internet Gateway
├── Subred privada 1 (AZ a):  10.0.10.0/24  ── NAT Gateway
└── Subred privada 2 (AZ b):  10.0.11.0/24  ── NAT Gateway
```

- **Públicas**: ALB (recibe tráfico HTTPS)
- **Privadas**: ECS Fargate + RDS (sin exposición directa a internet)

## Security Groups

| Recurso | Entrada | Origen |
|---------|---------|--------|
| ALB | 443/tcp | 0.0.0.0/0 |
| ECS | 8080/tcp | ALB security group |
| RDS | 5432/tcp | ECS security group |

## Alta disponibilidad

| Componente | Estrategia |
|------------|-----------|
| RDS | Multi-AZ: réplica síncrona en standby, failover automático < 2 min |
| ECS Fargate | Multi-AZ (2 subredes privadas), health check ALB cada 30s |
| ALB | Multi-AZ (2 subredes públicas), cross-zone load balancing |
| NAT Gateway | 1 instancia (AZ a). Para Multi-AZ completo: agregar segunda NAT en AZ b |

## Servicios AWS utilizados

| Servicio | Propósito | Costo mensual estimado |
|----------|-----------|:---:|
| RDS (t3.micro, 20GB, Multi-AZ) | PostgreSQL + PostGIS | ~$15 |
| ECS Fargate (256 CPU, 512 RAM) | Backend Go | ~$8 |
| ECR | Container registry | ~$1 |
| ALB | Balanceador de carga | ~$18 |
| S3 | Admin panel estático | ~$1 |
| CloudFront | CDN admin panel | ~$0 |
| Route 53 | DNS | ~$1 |
| Secrets Manager | Secretos encriptados | ~$1 |
| CloudWatch | Logs + métricas | ~$3 |
| ACM | Certificados TLS | $0 |
| **Total** | | **~$48/mo** |
