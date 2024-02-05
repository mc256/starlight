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

## Internet Gateway
resource "aws_internet_gateway" "ec2_igw" {
  vpc_id = aws_vpc.ec2_vpc.id

  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-igw"
    },
  )
}

## Route Table
resource "aws_route_table" "ec2_route_table" {
  vpc_id = aws_vpc.ec2_vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.ec2_igw.id
  }

  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-route-table"
    },
  )
}

resource "aws_route_table_association" "ec2_route_table_association" {
  subnet_id      = aws_subnet.ec2_subnet_public.id
  route_table_id = aws_route_table.ec2_route_table.id
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
  instance_type          = var.cloud_instance_type
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
    volume_size           = var.cloud_ebs_size_in_gb
    encrypted             = false
    delete_on_termination = true
  }


  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-cloud"
    },
  )

  user_data = <<-EOF
#!/bin/bash
echo "cloud" | sudo tee /etc/hostname > /dev/null
sudo hostname -F /etc/hostname
echo "10.0.1.21 cloud.cluster.local" | sudo tee -a /etc/hosts > /dev/null

sudo apt update && \
sudo apt upgrade -y && \
sudo apt install -y docker-compose git && \
sudo usermod -aG docker ubuntu && \
sudo systemctl enable docker && \
sudo systemctl start docker

cd /home/ubuntu && \
git clone https://github.com/mc256/starlight.git && \
cd /home/ubuntu/starlight && \
git checkout v${var.starlight_version} && \
cd /home/ubuntu/starlight/demo/compose/ && \
cp docker-compose-example.yaml docker-compose.yaml && \
docker-compose up -d

cat <<EOT | sudo tee -a /etc/sysctl.conf > /dev/null
net.core.wmem_max=125829120
net.core.rmem_max=125829120
net.ipv4.tcp_rmem= 10240 87380 125829120
net.ipv4.tcp_wmem= 10240 87380 125829120
net.ipv4.tcp_window_scaling = 1
net.ipv4.tcp_timestamps = 1
net.ipv4.tcp_sack = 1
net.ipv4.tcp_no_metrics_save = 1
net.core.netdev_max_backlog = 10000
EOT
sudo sysctl -p

touch /home/ubuntu/.completed
              EOF

}



resource "aws_instance" "starlight_edge" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.edge_instance_type
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
    volume_size           = var.edge_ebs_size_in_gb
    encrypted             = false
    delete_on_termination = true
  }


  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-edge"
    },
  )

  user_data = <<-EOF
#!/bin/bash
echo "edge" | sudo tee /etc/hostname > /dev/null
sudo hostname -F /etc/hostname
echo "10.0.1.21 cloud.cluster.local cloud" | sudo tee -a /etc/hosts > /dev/null

sudo apt update && sudo apt upgrade -y && \
sudo apt install -y build-essential containerd

sudo systemctl enable containerd  && \
sudo systemctl start containerd

wget https://go.dev/dl/go1.20.8.linux-amd64.tar.gz && \
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.20.8.linux-amd64.tar.gz

echo "export PATH=$PATH:/usr/local/go/bin" | sudo tee -a /home/ubuntu/.bashrc > /dev/null

export PATH=$PATH:/usr/local/go/bin
export GOPATH=/home/ubuntu/go
export HOME=/home/
source /home/ubuntu/.bashrc


cat <<EOT | sudo tee -a /etc/sysctl.conf > /dev/null
net.core.wmem_max=125829120
net.core.rmem_max=125829120
net.ipv4.tcp_rmem= 10240 87380 125829120
net.ipv4.tcp_wmem= 10240 87380 125829120
net.ipv4.tcp_window_scaling = 1
net.ipv4.tcp_timestamps = 1
net.ipv4.tcp_sack = 1
net.ipv4.tcp_no_metrics_save = 1
net.core.netdev_max_backlog = 10000
EOT
sudo sysctl -p


cd /home/ubuntu && \
git clone https://github.com/mc256/starlight.git && \
cd /home/ubuntu/starlight && \
git checkout v${var.starlight_version} && \
make starlight-daemon ctr-starlight && \
sudo make install install-systemd-service 

sudo systemctl enable starlight-daemon
sudo systemctl start starlight-daemon

sudo ctr-starlight add myproxy http cloud.cluster.local:8090

sudo mkdir /etc/containerd/ && \
cat <<EOT | sudo tee -a /etc/containerd/config.toml > /dev/null
  [proxy_plugins]
    [proxy_plugins.starlight]
      type = "snapshot"
      address = "/run/starlight/starlight-snapshotter.sock"
EOT

sudo systemctl restart containerd

touch /home/ubuntu/.completed
              EOF
}
