## EC2 Image
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

}

## create vpc
resource "aws_vpc" "ec2_vpc" {
  cidr_block = "10.0.0.0/16"

  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-vpc"
    },
  )
}

resource "aws_subnet" "ec2_subnet_public" {
  vpc_id                  = aws_vpc.ec2_vpc.id
  cidr_block              = "10.0.1.0/24"
  map_public_ip_on_launch = true

  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-subnet"
    },
  )
}


## Security Group
resource "aws_security_group" "ec2_security_group" {
  name        = "${local.project_name}-ec2-sg"
  description = "Allow inbound traffic from port 8080 and all outbound traffic"
  vpc_id      = aws_vpc.ec2_vpc.id

  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-sg"
    },
  )
}

resource "aws_vpc_security_group_egress_rule" "all_egress" {
  security_group_id = aws_security_group.ec2_security_group.id

  cidr_ipv4   = "0.0.0.0/0" # all
  ip_protocol = "-1"        # all

  description = "Allow all outbound traffic"
}

resource "aws_vpc_security_group_ingress_rule" "starlight_proxy_ingress" {
  security_group_id = aws_security_group.ec2_security_group.id

  cidr_ipv4   = "0.0.0.0/0"
  from_port   = 8090
  to_port     = 8090
  ip_protocol = "tcp"

  description = "Allow inbound traffic for Starlight Proxy"
}

resource "aws_vpc_security_group_ingress_rule" "adminer_ingress" {
  security_group_id = aws_security_group.ec2_security_group.id

  cidr_ipv4   = "0.0.0.0/0"
  from_port   = 8080
  to_port     = 8080
  ip_protocol = "tcp"

  description = "Allow inbound traffic for Adminer - Database management in a single PHP file"
}

resource "aws_vpc_security_group_ingress_rule" "registry_ingress" {
  security_group_id = aws_security_group.ec2_security_group.id

  cidr_ipv4   = "0.0.0.0/0"
  from_port   = 5000
  to_port     = 5000
  ip_protocol = "tcp"

  description = "Allow inbound traffic for Container Registry"
}

resource "aws_vpc_security_group_ingress_rule" "ssh_ingress" {
  security_group_id = aws_security_group.ec2_security_group.id

  cidr_ipv4   = "0.0.0.0/0"
  from_port   = 22
  to_port     = 22
  ip_protocol = "tcp"

  description = "Allow inbound traffic for Container Registry"
}


## Key Pair
resource "aws_key_pair" "deployer" {
  count      = var.ssh_public_key != "" ? 1 : 0
  key_name   = var.ssh_key_name
  public_key = var.ssh_public_key
}


## EC2 Host
resource "aws_instance" "starlight_cloud" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.ec2_subnet_public.id
  key_name               = var.ssh_public_key == "" ? var.ssh_key_name : aws_key_pair.deployer[0].key_name
  vpc_security_group_ids = [aws_security_group.ec2_security_group.id]
  monitoring             = true
  private_ip             = "10.0.1.21"

  lifecycle {
    ignore_changes = [subnet_id, ami]
  }

  root_block_device {
    volume_type           = "gp3"
    volume_size           = var.ebs_size_in_gb
    encrypted             = false
    delete_on_termination = true
  }


  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-cloud"
    },
  )
}



resource "aws_instance" "starlight_edge" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.ec2_subnet_public.id
  key_name               = var.ssh_public_key == "" ? var.ssh_key_name : aws_key_pair.deployer[0].key_name
  vpc_security_group_ids = [aws_security_group.ec2_security_group.id]
  monitoring             = true
  private_ip             = "10.0.1.22"

  lifecycle {
    ignore_changes = [subnet_id, ami]
  }

  root_block_device {
    volume_type           = "gp3"
    volume_size           = var.ebs_size_in_gb
    encrypted             = false
    delete_on_termination = true
  }


  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-edge"
    },
  )
}
