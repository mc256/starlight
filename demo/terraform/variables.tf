variable "default_tags" {
  default = {
    project     = "starlight-experiment"
    environment = "dev"
  }
}

variable "ssh_key_name" {
  type        = string
  description = "the ssh key to access the ec2 instance"
}

variable "ssh_public_key" {
  type        = string
  description = "the private key to access the ec2 instance. If not provided, we assume the public key is already in the AWS account"
  default     = ""
}

variable "cloud_instance_type" {
  type        = string
  default     = "m5a.large"
  description = "the instance type to use"
}

variable "edge_instance_type" {
  type        = string
  default     = "t2.micro"
  description = "the instance type to use"
}

variable "project_id" {
  type        = string
  default     = "starlight"
  description = "the project name"
}

variable "cloud_ebs_size_in_gb" {
  type        = number
  default     = 20
  description = "the ebs size in gb"
}

variable "edge_ebs_size_in_gb" {
  type        = number
  default     = 10
  description = "the ebs size in gb"
}


variable "starlight_version" {
  type        = string
  default     = "0.6.2"
  description = "the version of the starlight software to deploy"
}
