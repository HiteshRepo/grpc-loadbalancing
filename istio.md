- curl -L https://istio.io/downloadIstio | sh -

- export PATH="$PATH:/home/hitesh/istio-1.18.0/bin"

- istioctl install

- kubectl get ns | grep istio
istio-system      Active   3m32s

- kubectl label ns default istio-injection=enabled

- kubectl get ns default --show-labels 
NAME      STATUS   AGE   LABELS
default   Active   78d   istio-injection=enabled,kubernetes.io/metadata.name=default

- addons: kubectl apply -f /home/hitesh/istio-1.18.0/samples/addons/

- kubectl get svc -n istio-system