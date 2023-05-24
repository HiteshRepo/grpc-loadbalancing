## gRPC

gRPC has many benefits, like:
1. Multiplexes many requests using same connection.
2. Support for typical client-server request-response as well as duplex streaming.
3. Usage of a fast, very light, binary protocol with structured data as the communication medium among services.

[More about gRPC](https://www.infracloud.io/blogs/understanding-grpc-concepts-best-practices/)

All above make gRPC a very attractive deal but there is some consideration with gRPC particularly load balancing.

## The issue

Lets delve deep into the issue.

For this we will require a setup. The setup includes below:
- a gRPC server, we call it `Greet Server`.
- a client that acts as a REST gateway and internally it is a gRPC client as well. We call it `Greet Client`.

We are also using kubernetes for the demonstration, hence there are a bunch of YAML manifest files. Let me explain them below:

<details> <summary> greetserver-deploy.yaml </summary>

```yml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: greetserver-deploy
spec:
  replicas: 3
  selector:
    matchLabels:
      run: greetserver
  template:
    metadata:
      labels:
        run: greetserver
    spec:
      containers:
        - image: hiteshpattanayak/greet-server:1.0
          imagePullPolicy: IfNotPresent
          name: greetserver
          ports:
            - containerPort: 50051
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
```
</details>

The above is a deployment mainfest of `Greet Server`, that spins up 3 replicas of `Greet Server`.
The `Greet Server` uses `hiteshpattanayak/greet-server:1.0` image.
Also each pod of the deployment exposes `50051` port.
Environment variables: POD_IP and POD_NAME are injected into the pods.

What does each pod in the above server do?

They expose an `rpc` or service that expects a `first_name` and a `last_name`, in response they return a message in this format:
`reponse from Greet rpc: Hello, <first_name> <last_name> from pod: name(<pod_name>), ip(<pod_ip>).`

From the response, we can deduce which pod did our request land in.

<details> <summary> greet.svc.yaml </summary>

```yml
apiVersion: v1
kind: Service
metadata:
  labels:
    run: greetserver
  name: greetserver
  namespace: default
spec:
  ports:
    - name: grpc
      port: 50051
      protocol: TCP
      targetPort: 50051
  selector:
    run: greetserver
```
</details>

The above is a service manifest of `Greet server service`. This basically acts as a proxy to above `Greet Server` pods.

The `selector` section of the service matches with the `labels` section of each pod.

<details> <summary> greetclient-deploy.yaml </summary>

```yml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: greetclient-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      run: greetclient
  template:
    metadata:
      labels:
        run: greetclient
    spec:
      containers:
        - image: hiteshpattanayak/greet-client:4.0
          name: greetclient
          ports:
            - containerPort: 9091
          env:
            - name: GRPC_SERVER_HOST
              value: greetserver.default.svc.cluster.local
            - name: GRPC_SVC
              value: greetserver
            - name: POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
```

</details>

The above is a deployment mainfest of `Greet Client`, that spins up 1 replica of `Greet Client`.

As mentioned above the pod runs an applications that acts as a rest gateway and reaches out to `Greet Server` in order to process the request.

This deployment is using `hiteshpattanayak/greet-client:4.0` image.

The `4.0` tagged image has the load balancing issue.

Also the pod(s) expose port `9091`.

<details> <summary> greetclient-svc.yaml </summary>

```yml
apiVersion: v1
kind: Service
metadata:
  labels:
    run: greetclient
  name: greetclient
  namespace: default
spec:
  ports:
    - name: restgateway
      port: 9091
      protocol: TCP
      targetPort: 9091
  selector:
    run: greetclient
  type: LoadBalancer
```

</details>

The above service is just to redirect traffic to the `Greet Client` pods.

<details> <summary> greet-ingress.yaml </summary>

```yml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: greet-ingress
  namespace: default
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "false"
spec:
  rules:
    - host: greet.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: greetclient
                port:
                  name: restgateway
```

</details>

The above ingress is to expose `Greet Client Service` to outside of the cluster.

Note:
`minikube` by default does not have ingress enabled by default
- check enabled or not: `minikube addons list`
- enable ingress addon: `minikube addons enable ingress`

<details> <summary> greet-clusterrole.yaml </summary>

```yml

kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: default
  name: service-reader
rules:
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["get", "watch", "list"]
```

</details>

<details> <summary> greet-clusterrolebinding.yaml </summary>

```yml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: service-reader-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: service-reader
subjects:
  - kind: ServiceAccount
    name: default
    namespace: default
```

</details>

The cluster role and cluster role binding are required because the `default` service account does not have permission to fetch service details.
And the greet client pod internally tries to fetch service details, hence the binding is required.

Create the setup in below sequence:

```bash

kubectl create -f greet-clusterrole.yaml

kubectl create -f greet-clusterrolebinding.yaml

kubectl create -f greetserver-deploy.yaml

kubectl get po -l 'run=greetserver' -o wide
<<com
NAME                                  READY   STATUS    RESTARTS   AGE   IP           NODE       NOMINATED NODE   READINESS GATES
greetserver-deploy-7595ccbdd5-67bmd   1/1     Running   0          91s   172.17.0.4   minikube   <none>           <none>
greetserver-deploy-7595ccbdd5-k6zbl   1/1     Running   0          91s   172.17.0.3   minikube   <none>           <none>
greetserver-deploy-7595ccbdd5-l8kmv   1/1     Running   0          91s   172.17.0.2   minikube   <none>           <none>
com

kubectl create -f greet.svc.yaml
kubectl get svc
<<com
NAME          TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)     AGE
greetserver   ClusterIP   None         <none>        50051/TCP   77s
com

kubectl create -f greetclient-deploy.yaml
kubectl get po -l 'run=greetclient' -o wide
<<com
NAME                                 READY   STATUS    RESTARTS   AGE   IP           NODE       NOMINATED NODE   READINESS GATES
greetclient-deploy-6bddb94df-jwr25   1/1     Running   0          35s   172.17.0.6   minikube   <none>           <none>
com

kubectl create -f greet-client.svc.yaml
kubectl get svc
<<com
NAME          TYPE           CLUSTER-IP       EXTERNAL-IP   PORT(S)          AGE
greetclient   LoadBalancer   10.110.255.115   <pending>     9091:32713/TCP   22s
greetserver   ClusterIP      None             <none>        50051/TCP        5m14s
com

kubectl create -f greet-ingress.yaml
kubectl get ingress
<<com
NAME            CLASS   HOSTS       ADDRESS        PORTS   AGE
greet-ingress   nginx   greet.com   192.168.49.2   80      32s
com
```

since we have exposed the `Greet Client` to outside of cluster via `greet-ingress`, the endpoint can be accessed on: `http://greet.com/greet`.
so when we make a curl request:

<details> <summary> Request#1 </summary>

```bash
curl --request POST \
  --url http://greet.com/greet \
  --header 'Content-Type: application/json' \
  --data '{
	"first_name": "Hitesh",
	"last_name": "Pattanayak"
}'

<<com
Response

reponse from Greet rpc: Hello, Hitesh Pattanayak from pod: name(greetserver-deploy-7595ccbdd5-l8kmv), ip(172.17.0.2).
com
```

</details>

<details> <summary> Request#2 </summary>

```bash
curl --request POST \
  --url http://greet.com/greet \
  --header 'Content-Type: application/json' \
  --data '{
	"first_name": "Hitesh",
	"last_name": "Pattanayak"
}'

<<com
Response

reponse from Greet rpc: Hello, Hitesh Pattanayak from pod: name(greetserver-deploy-7595ccbdd5-l8kmv), ip(172.17.0.2).
com
```

</details>

<details> <summary> Request#3 </summary>

```bash
curl --request POST \
  --url http://greet.com/greet \
  --header 'Content-Type: application/json' \
  --data '{
	"first_name": "Hitesh",
	"last_name": "Pattanayak"
}'

<<com
Response

reponse from Greet rpc: Hello, Hitesh Pattanayak from pod: name(greetserver-deploy-7595ccbdd5-l8kmv), ip(172.17.0.2).
com
```

</details>

So the ISSUE is no matter hw many request I make, the request lands up in the same server. This is happending because of sticky nature of HTTP/2.
The advantage of gRPC becomes it own peril.

The codebase to replicate the issue can be found [here](https://github.com/HiteshRepo/grpc-loadbalancing/commit/dd31d2628d4ee1e47b07b5737ff51cfc43c76d4e).

In the upcoming post we shall discuss on how to achieve client side load balancing.