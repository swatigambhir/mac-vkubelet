# mac-vkubelet

Mac provider for virtual kubelet under development.

Proof-of-concept implementation of Virtual Kubelet Provider for macos that can be deployed in kubernetes cluster with skaffold or helm chart.

The mac virtual kubelet provider can be used to manage on-prem mac hosts from kubectl API enabling scripting, auto-scaling and monitoring, config management and possible integration with a web UI for healthcheck without the need of actual virtualization or performace impact. 

Ref: https://github.com/virtual-kubelet/virtual-kubelet


# How to test with minikube setup
# Install minikube
brew install hyperkit
brew install minikube
# Start minikube locally
minikube start --v=7 --alsologtostderr
# Run Virtual kublet with kubeconfig
./mac-vkubelet --kubeconfig /Users/sgambhir/.kube/config --nodename mac-test

# Run kubectl Get Nodes 
sgambhir-mn3:code sgambhir$ kubectl get nodes
NAME                  STATUS   ROLES           AGE     VERSION
mac-test              Ready    worker          15s     
minikube              Ready    control-plane   4h44m   v1.24.3

# Create Pod
kubectl create -f /Users/sgambhir/code/go/src/mac-vkubelet/internal/sample-pod.yaml
kubectl describe pod/test-pod-create
kubectl delete pod/test-pod-create