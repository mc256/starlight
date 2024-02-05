# Setup Starlight Experiment using Terraform

## Prerequisites
- [Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli)
- [AWS account](https://aws.amazon.com/) You will need to setup programmatic access to AWS (e.g. set up credentials in `$HOME/.aws/config` and `$HOME/.aws/credentials`).


## Install
1. Clone the repository
    ```shell
    git clone https://github.com/mc256/starlight.git
    cd starlight/demo/terraform
    ```

2. Initialize Terraform
    ```shell
    terraform init
    ```

3. Modify `terraform.tfvars` to your needs. 


4. Apply the configuration
    ```shell
    terraform apply
    ```

5. Wait for the infrastructure to be created. This may take a few minutes. After the infrastructure is create you can see there is a `.completed` file in the home directory.

## Experiment

1. SSH into the Starlight CLI Tool pods in the edge node.
    ```shell
    ssh -i <path-to-ssh-key> ubuntu@<edge-node-ip>
    ```

2. Run the experiment



## Uninstall

1. Destroy the infrastructure
    ```shell
    terraform destroy
    ```