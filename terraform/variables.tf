variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "project" {
  description = "Project name for resource tagging"
  type        = string
  default     = "umi"
}

variable "environment" {
  description = "Environment (production, staging)"
  type        = string
  default     = "production"
}

variable "domain" {
  description = "Root domain for Route 53"
  type        = string
  default     = "umi.app"
}

variable "db_name" {
  description = "Database name"
  type        = string
  default     = "umi"
}

variable "db_username" {
  description = "Database master username"
  type        = string
  default     = "umi_admin"
}
