go mod init keit
go mod tidy

go build -o keit .   

CGO_ENABLED=0 GOOS=linux go build -o keit .
docker build -t keit .

# with docker images, you will find the new keit image.
# now tag it and push it to ecr.

docker tag keit 623566434957.dkr.ecr.eu-west-1.amazonaws.com/keit:latest

aws aws ecr get-login-password --region eu-west-1 | docker login --username AWS --password-stdin 623566434957.dkr.ecr.eu-west-1.amazonaws.com

docker push 623566434957.dkr.ecr.eu-west-1.amazonaws.com/keit:latest



-----
get the pod to run:
keit_deployment.yaml  (TODO it runs now in default namespace)

Add the role bindings otherwise we do not have access TODO, is this good its now wide open?
clusterrole-node-reader.yaml
clusterrole.yaml

clusterrolebinding-node-reader.yaml
clusterrolebinding.yaml

