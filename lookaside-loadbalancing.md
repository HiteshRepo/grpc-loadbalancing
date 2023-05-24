## gRPC lookaside load balancing

Earlier we discussed about:
- Load balancing challenge with gRPC
- How to address the above challenge via client side load balancing

Even though we were able to resolve the load balancing issue but we traded off one of the major advantage of gRPC which is long duration connections.

So in this post we would like the achive load balancing (still client side) but we are gonna not trade off the above mentionde gRPC's advantage.

I would like to re-iterate when I say onus to load balance falls on `client side`, client does not mean end user. All gRPC servers have a REST gateway that is used by end users. gRPC services are not directly exposed because of lack of browser support.

## Lookaside load balancer

The purpose of this load balancer is to resolve which gRPC server to connect.

At the moment this load balancer works in two ways: round robin and random.

Load balancer itself is gRPC based and since the load is not going to be too much only one pod would suffice.

It exposes a service called `lookaside` and an rpc called `Resolve` which expects the type of routing along with some details about the gRPC servers like kubernetes service name and namespace they exist in.

Using the service name and namespace, it is going to fetch kubernetes endpoints object associated with it. From the endpoint object server IPs can be found.
Those IPs are going to be stored in memory. Every now and then those IPs would be refreshed based on interval set. For every request to resolve IP, it is going to rotate the IPs based on the routing type in the request.

Code for lookaside load balancer can be found [here](https://github.com/HiteshRepo/grpc-loadbalancing/tree/lookaside/internal/app/lookaside).

We are using the image `hiteshpattanayak/lookaside:9.0` for lookaside pod.

The pod manifest would be like this:

```yml
apiVersion: v1
kind: Pod
metadata:
  labels:
    run: lookaside
  name: lookaside
  namespace: default
spec:
  containers:
    - image: hiteshpattanayak/lookaside:9.0
      name: lookaside
      ports:
        - containerPort: 50055
      env:
        - name: LB_PORT
          value: "50055"
```

since it is too a gRPC server, the exposed port is `50055`.

The service manifest that exposes the pod is as below:

```yml
apiVersion: v1
kind: Service
metadata:
  labels:
    run: lookaside-svc
  name: lookaside-svc
  namespace: default
spec:
  ports:
    - port: 50055
      protocol: TCP
      targetPort: 50055
  selector:
    run: lookaside
  clusterIP: None
```

I chose `headless` service for this as well but there is no such need for this.

Updated the `ClusterRole` to include ability to fetch `endpoints` and `pod` details

```yml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: default
  name: service-reader
rules:
  - apiGroups: [""]
    resources: ["services", "pods", "endpoints"]
    verbs: ["get", "watch", "list"]
```

## Changes with Greet Client

`Greet Client` is now [integrated](https://github.com/HiteshRepo/grpc-loadbalancing/blob/177c0fdccad06a76d7d6ce221ee267a47244dc43/internal/app/greetclient/app.go#L38) with lookaside loadbalancer.

The client is [set](https://github.com/HiteshRepo/grpc-loadbalancing/blob/177c0fdccad06a76d7d6ce221ee267a47244dc43/internal/app/greetclient/app.go#L131) to use `RoundRobin` routing type but can be made configurable via configmap or environment variables.

Removed setting `load-balancing` policy and forcefully terminating connection by setting `WithBlock` option while dialing.

from
```go
conn, err := grpc.Dial(
    servAddr,
    grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
    grpc.WithBlock(),
    opts,
)
```

to
```go
conn, err := grpc.Dial(
    servAddr,
    opts,
)
```

So how does it solve the earlier load balancing problem where we traded off terminating long duration connections for the sake of load balancing.

What we did was to store the previous connections to the server and reuse it but rotate for each request.

```go
if c, ok := a.greetClients[host]; !ok {
    servAddr := fmt.Sprintf("%s:%s", host, serverPort)

    fmt.Println("dialing greet server", servAddr)

    conn, err := grpc.Dial(
        servAddr,
        opts,
    )
    if err != nil {
        log.Printf("could not connect greet server: %v", err)
        return err
    }

    a.conn[host] = conn

    a.currentGreetClient = proto.NewGreetServiceClient(conn)
    a.greetClients[host] = a.currentGreetClient
} else {
    a.currentGreetClient = c
}
```

so this wraps up the 3-fold-posts of gRPC load balancing.