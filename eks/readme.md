
To create the EKS cluster, from the commandline:
eksctl create cluster -f cluster-config.yaml

-----
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
Deploy boavizta: via deployment_boavizta.yaml
This local pod is used by keit to the the embodied carbon of the servers

-----
get the pod to run:
keit_deployment.yaml

Add the role bindings otherwise we the keit.go does not have access to readout all the pods on and all the nodes (runs only in keit namespace):
for the pods:
clusterrole.yaml
clusterrolebinding.yaml

for the nodes:
clusterrole-node-reader.yaml
clusterrolebinding-node-reader.yaml

-----
Usage, examples:

Grafana:
kubectl -n prometheus port-forward svc/prometheus-grafana 3000:80 &
(check in browser, username, password)

Prometheus:
kubectl -n prometheus port-forward svc/prometheus-operated 9090 &
(check browser)

check the keit metrics, embodied/embedded value of instances.
kubectl -n keit port-forward service/keit-service 8080:8080 &
curl -s http://localhost:8080/metrics
