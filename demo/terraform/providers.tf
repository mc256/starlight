terraform {
  required_providers {
    aws = {
      version = "~> 5.35.0"
      source  = "hashicorp/aws"
    }
  }
}

provider "aws" {
  profile = "default"
  # Hard-coded credentials are not recommended in any Terraform configuration and 
  # risks secret leakage should this file ever be committed to a public version control system.
  # Instead, use environment variables or shared credentials file
  # ref: https://registry.terraform.io/providers/-/aws/latest/docs#environment-variables

  # Please set the credentials in the environment variables or in $HOME/.aws/credentials and $HOME/.aws/config files
}
