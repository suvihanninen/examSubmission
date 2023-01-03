package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"strings"

	// this has to be the same as the go.mod module,
	// followed by the path to the folder the proto file is in.
	distSystem "github.com/suvihanninen/examSubmission.git/grpc"
	"google.golang.org/grpc"
)

var server distSystem.DistServiceClient //the server
var ServerConn *grpc.ClientConn         //the server connection
var port string
var clientNumber string

func main() {
	clientNumber = os.Args[1]
	port = ":5001"
	connection, err := grpc.Dial(port, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Unable to connect: %v", err)
	}

	//log to file instead of console
	f := setLogClient()
	defer f.Close()

	server = distSystem.NewDistServiceClient(connection) //creates a new client

	defer connection.Close()

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		println("Enter 'add + [word] + [definition]' (each one word) or 'read + [word]'")
		for {
			scanner.Scan()
			text := scanner.Text()

			if strings.Contains(text, "add") {
				input := strings.Fields(text)

				word := input[1]

				definition := input[2]

				addWord := &distSystem.AddRequest{
					Word:       string(word),
					Definition: string(definition),
				}

				ack := Add(addWord, connection, server, port)

				log.Printf("Client %s: Add response: %v", clientNumber, ack)
				println("Client "+clientNumber+": Add response: ", ack)

			} else if strings.Contains(text, "read") {
				input := strings.Fields(text)
				word := input[1]
				readRequest := &distSystem.ReadRequest{Word: word}

				log.Printf("Client %s: sent a Read request", clientNumber)
				definition := Read(readRequest, connection, server, port)

				log.Printf("Client %s: Result from Read request: %s", clientNumber, definition)
				println("Client " + clientNumber + ": Result from Read request: " + definition)

			} else {
				println("Sorry didn't quite understand, try again ")
			}

		}
	}()

	for {

	}

}

func Add(AddRequest *distSystem.AddRequest, connection *grpc.ClientConn, server distSystem.DistServiceClient, port string) bool {
	var acknowledgement bool
	log.Printf("Client %s: sent an add request ", clientNumber)
	response, err := server.Add(context.Background(), AddRequest)
	if err != nil {
		log.Printf("Client %s: Add failed: ", clientNumber, err)
		log.Printf("Client %s: PrimaryReplica has died", clientNumber)
		connection, server = Redial(port)
		acknowledgement = Add(AddRequest, connection, server, port)
		return acknowledgement
	}
	acknowledgement = response.GetAck()
	return acknowledgement
}

func Read(ReadRequest *distSystem.ReadRequest, connection *grpc.ClientConn, server distSystem.DistServiceClient, port string) string {

	var definition string
	response, err := server.Read(context.Background(), ReadRequest)
	if err != nil {
		log.Printf("Client %s: Read request failed: ", clientNumber, err)
		connection, server = Redial(port)
		definition = Read(ReadRequest, connection, server, port)
		return definition
	}
	definition = response.GetDefinition()
	return definition
}

func Redial(port string) (*grpc.ClientConn, distSystem.DistServiceClient) {
	log.Printf("Client: PrimaryReplica on port %s is not listening anymore. It has died", port)
	newPort := ":5002"
	port = ":5002"
	log.Printf("Client %s: Redialing to new port: %s", clientNumber, newPort)
	connection, err := grpc.Dial(newPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Unable to connect: %v", err)
	}

	server = distSystem.NewDistServiceClient(connection) //creates a new client

	log.Printf("Client %s: Client has connected to new PrimaryReplica on port %s", clientNumber, newPort)
	return connection, server
}

// sets the logger to use a log.txt file instead of the console
func setLogClient() *os.File {
	f, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	return f
}
