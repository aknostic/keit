#!/usr/bin/env bash
echo -e "Usage: . kuberole.sh <set|unset>"
echo -e "(The '.' is part of the command)"
if [ $# -eq 0 ]
  then
    cmd=set
  else
    cmd=$1; shift
fi
# Check if dependencies are installed
command -v aws >/dev/null 2>&1 || { echo >&2 "I require aws-cli but it's not installed. Exiting."; exit 3; }
command -v jq >/dev/null 2>&1 || { echo >&2 "I require jq but it's not installed. Exiting."; exit 3; }
# Execute
case $cmd in
  set)
    unset AWS_ACCESS_KEY_ID
    unset AWS_SECRET_ACCESS_KEY
    unset AWS_SESSION_TOKEN
    unset KUBECONFIG
    account_id=$(aws sts get-caller-identity --output text --query 'Account')
    # kubectl_role=$(aws sts assume-role --role-arn "arn:aws:iam::${account_id}:role/shared-eks-management-role" --role-session-name "kubectl-session")
    kubectl_role=$(aws sts assume-role --role-arn "arn:aws:iam::${account_id}:role/eksworkshop-admin" --role-session-name "kubectl-session")
    cluster_config=$(echo $AWS_PROFILE | cut -d- -f2,3)
    mkdir -p ~/.kube/config_files
    export AWS_ACCESS_KEY_ID=$(echo $kubectl_role | jq .Credentials.AccessKeyId | xargs)
    export AWS_SECRET_ACCESS_KEY=$(echo $kubectl_role | jq .Credentials.SecretAccessKey | xargs)
    export AWS_SESSION_TOKEN=$(echo $kubectl_role | jq .Credentials.SessionToken | xargs)
    export KUBECONFIG=~/.kube/config_files/$cluster_config
    aws eks update-kubeconfig --name KEIT --kubeconfig ~/.kube/config_files/$cluster_config
    ;;
  unset)
    unset AWS_ACCESS_KEY_ID
    unset AWS_SECRET_ACCESS_KEY
    unset AWS_SESSION_TOKEN
    unset KUBECONFIG
    ;;
  *)
    echo "Invalid command specified"
    exit 1
    ;;
esac
