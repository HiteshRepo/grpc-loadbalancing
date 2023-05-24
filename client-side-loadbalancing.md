## gRPC Client side load balancing

We have discussed earlier about one of the challenges with gRPC which is load balancing.

That happens due to the sticky nature of gRPC connections.

Now we shall discuss how to resolve the issue.

This particular solution is quite simple.

The onus to load balance falls on the client itself.

To be particular, client does not mean end user. All gRPC servers have a REST gateway that is used by end users.

This is because HTTP2, which is the protocol used by gRPC, is yet to have browser support.

Hence the REST gateway acts as a gRPC client to gRPC servers. And thats why gRPC is mostly used for internal communications. 

Earlier we had used `hiteshpattanayak/greet-client:4.0` image for `Greet Client` which had the normal gRPC setup without client side load balancing.
The code can be referred [here](https://github.com/HiteshRepo/grpc-loadbalancing/commit/dd31d2628d4ee1e47b07b5737ff51cfc43c76d4e).

## Changes

### Code changes

For this solution we use `hiteshpattanayak/greet-client:11.0` image. The [codebase](https://github.com/HiteshRepo/grpc-loadbalancing/pull/1/files) has below changes:

Updated client deployment manifest:

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
        - image: hiteshpattanayak/greet-client:11.0
          name: greetclient
          ports:
            - containerPort: 9091
          env:
            - name: GRPC_SVC
              value: greetserver
            - name: POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
```

- configuring load balancing policy while making dialing to the server.
- configuring to terminate connection while dialing to the server.

```go
a.conn, err = grpc.Dial(
		servAddr,
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
		grpc.WithBlock(),
		opts,
	)
```

- the server address used while dialing needs to the dns address of the server.

```go
var serverHost string
if host := kubernetes.GetServiceDnsName(client, os.Getenv("GRPC_SVC"), os.Getenv("POD_NAMESPACE")); len(host) > 0 {
		serverHost = host
	}

servAddr := fmt.Sprintf("%s:%s", serverHost, serverPort)
```

### Headless service

- also earlier while replicating the issue the service (greetserver) we created for `Greet server pods` was of normal `ClusterIP` type. The headless ClusterIP service is required for this solution.

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
  clusterIP: None
```

One significant thing to notice over here is that this is a special type of `ClusterIP` service called `Headless` service.

In this `service` kind, the type of service is not specified. By default the type becomes `ClusterIP`. Which means the service becomes available within cluster.

You can set `.spec.clusterIP`, if you already have an existing DNS entry that you wish to reuse.

In case you set `.spec.clusterIP` to `None`, it makes the service `headless`, which means when a client sends a request to a headless Service, it will get back a list of all Pods that this Service represents (in this case, the ones with the label `run: greetserver`). 

Kubernetes allows clients to discover pod IPs through DNS lookups. Usually, when you perform a DNS lookup for a service, the DNS server returns a single IP — the service’s cluster IP. But if you tell Kubernetes you don’t need a cluster IP for your service (you do this by setting the clusterIP field to None in the service specification ), the DNS server will return the pod IPs instead of the single service IP. Instead of returning a single DNS A record, the DNS server will return multiple A records for the service, each pointing to the IP of an individual pod backing the service at that moment. Clients can therefore do a simple DNS A record lookup and get the IPs of all the pods that are part of the service. The client can then use that information to connect to one, many, or all of them.

Basically, the Service now lets the client decide on how it wants to connect to the Pods.

#### Verify headless service DNS lookup

create the headless service:
```bash
kubectl create -f greet.svc.yaml
```

create an utility pod:
```bash
kubectl run dnsutils --image=tutum/dnsutils --command -- sleep infinity
```

verify by running `nslookup` command on the pod
```bash
kubectl exec dnsutils --  nslookup greetserver

<<com
Result

Server:         10.96.0.10
Address:        10.96.0.10#53
Name:   greetserver.default.svc.cluster.local
Address: 172.17.0.4
Name:   greetserver.default.svc.cluster.local
Address: 172.17.0.3
Name:   greetserver.default.svc.cluster.local
Address: 172.17.0.2
```

As you can see headless service resolves into the IP address of all pods connected through service. 

Contrast this with the output returned for non-headless service.

```bash
kubectl exec dnsutils --  nslookup greetclient

<<com
Server:		10.96.0.10
Address:	10.96.0.10#53

Name:	greetclient.default.svc.cluster.local
Address: 10.110.255.115
com
```

Now lets test the changes by making curl requests to the exposed ingress.

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

reponse from Greet rpc: Hello, Hitesh Pattanayak from pod: name(greetserver-deploy-7595ccbdd5-k6zbl), ip(172.17.0.3).
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

reponse from Greet rpc: Hello, Hitesh Pattanayak from pod: name(greetserver-deploy-7595ccbdd5-67bmd), ip(172.17.0.4).
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

The issue no longer exists.

But what we are losing here is the capability of gRPC to retain connections for a longer period of time and multiplex several requests through them thereby reducing latency.

In the upcoming post we shall discuss on how to still retain the connections as well as achieve load balancing.