package greetclient

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/hiteshrepo/grpc-loadbalancing/internal/pkg/proto"
	"google.golang.org/grpc"
)

type App struct {
	client proto.GreetServiceClient
	conn   *grpc.ClientConn
}

type GreetingRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (a *App) Start() {
	err := a.setupGreetClient()
	if err != nil {
		log.Printf("could not setup greet client: %v", err)
		return
	}

	fmt.Println("Starting gateway")

	router := gin.Default()
	router.POST("/greet", a.greet)

	port := "9091"
	if len(os.Getenv("GATEWAY_PORT")) > 0 {
		port = os.Getenv("GATEWAY_PORT")
	}

	fmt.Println("Starting REST Gateway")

	router.Run(fmt.Sprintf("0.0.0.0:%s", port))
}

func (a *App) setupGreetClient() error {
	var err error

	fmt.Println("Starting greet client")

	opts := grpc.WithInsecure()
	serverHost := "localhost"
	serverPort := "50051"

	if host, ok := os.LookupEnv("GRPC_SERVER_HOST"); ok {
		serverHost = host
	}

	if port, ok := os.LookupEnv("GRPC_SERVER_PORT"); ok {
		serverPort = port
	}

	servAddr := fmt.Sprintf("%s:%s", serverHost, serverPort)

	fmt.Println("dialing", servAddr)

	a.conn, err = grpc.Dial(
		servAddr,
		opts,
	)
	if err != nil {
		log.Printf("could not connect: %v", err)
		return err
	}

	a.client = proto.NewGreetServiceClient(a.conn)

	return nil
}

func (a *App) Shutdown() {
	a.conn.Close()
}

func (a *App) doUnary(firstName, lastName string) string {
	req := &proto.GreetingRequest{
		Greeting: &proto.Greeting{
			FirstName: firstName,
			LastName:  lastName,
		},
	}
	res, err := a.client.Greet(context.Background(), req)
	if err != nil {
		log.Fatalf("error while calling greet rpc : %v", err)
	}
	return fmt.Sprintf("reponse from Greet rpc: %v", res.Result)
}

func (a *App) greet(c *gin.Context) {
	fmt.Println("got request - REST Gateway")

	var gr GreetingRequest
	if err := c.BindJSON(&gr); err != nil {
		return
	}

	resp := a.doUnary(gr.FirstName, gr.LastName)
	c.String(http.StatusOK, resp)
}
