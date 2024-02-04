variable "default_tags" {
  default = {
    project     = "starlight-experiment"
    environment = "dev"
  }
}

variable "ssh_keyname" {
  type        = string
  description = "the ssh key to access the ec2 instance"
}

variable "instance_type" {
  type        = string
  default     = "t3a.nano"
  description = "the instance type to use"
}

variable "project_id" {
  type        = string
  default     = "starlight"
  description = "the project name"
}

variable "ebs_size_in_gb" {
  type        = number
  default     = 10
  description = "the ebs size in gb"
}

