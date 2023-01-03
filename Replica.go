package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	// this has to be the same as the go.mod module,
	// followed by the path to the folder the proto file is in.
	distSystem "github.com/suvihanninen/examSubmission.git/grpc"
	"google.golang.org/grpc"
)

type Server struct {
	distSystem.UnimplementedDistServiceServer
	id         int32
	peers      map[int32]distSystem.DistServiceClient
	isPrimary  bool
	ctx        context.Context
	time       time.Time
	primary    distSystem.DistServiceClient
	dictionary map[string]string
	lock       chan bool
}

func main() {

	portInput, _ := strconv.ParseInt(os.Args[1], 10, 32) //Takes arguments 0 and 1
	primary, _ := strconv.ParseBool(os.Args[2])
	ownPort := int32(portInput) + 5001
	println("primary?: "+strconv.FormatBool(primary)+" port?: ", ownPort)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := &Server{
		id:         ownPort,
		peers:      make(map[int32]distSystem.DistServiceClient),
		ctx:        ctx,
		isPrimary:  primary,
		dictionary: make(map[string]string),
		primary:    nil,
		lock:       make(chan bool, 1),
	}

	//log to file instead of console
	f := setLogRMServer()
	defer f.Close()

	//unlock
	server.lock <- true

	//Primary needs to listen so that replica managers can ask if it's alive
	//Replica managers need to listen for incoming data to be replicated
	println("Making connection in order to listen other replicas")
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	distSystem.RegisterDistServiceServer(grpcServer, server)
	println("Server listening on port: ", ownPort)
	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v", err)
		}
	}()
	println("Connecting to other peers")
	for i := 0; i < 2; i++ {
		port := int32(5001) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		log.Printf("Server %v: Trying to dial: %v\n", server.id, port)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock()) //This is going to wait until it receives the connection
		if err != nil {
			log.Fatalf("Could not connect: %s", err)
		}
		defer conn.Close()
		c := distSystem.NewDistServiceClient(conn)
		server.peers[port] = c
		if port == 5001 {
			log.Printf("We have assigned primary replica on port: ", port)
			server.primary = c
		}
	}
	println("Connected to the peers")
	// If server is primary, dial to all other replica managers
	if server.isPrimary {
		println("Waiting for heartbeat request")
	} else {
		println("Sending heartbeat requests to PR")
		go func() {
			for {
				time.Sleep(2 * time.Second)
				heartbeatMsg := &distSystem.BeatRequest{Message: "alive?"}
				_, err := server.primary.GetHeartBeat(server.ctx, heartbeatMsg)
				if err != nil {
					log.Printf("Backup Replica %v: Something went wrong while sending heartbeat", server.id)
					log.Printf("Backup Replica %v: Error:", server.id, err)
					log.Printf("Backup Replica %v: Exception, We did not get heartbeat back from Primary Replica with port %v. It has died, ", server.id, 5001)
					delete(server.peers, 5001)
					server.ElectLeader()
				}
				//comment out if you wanna see the logging of heartbeat
				//log.Printf("We got a heart beat from %s", response)

				if server.isPrimary {
					break
				}
			}
		}()
		for {

		}
	}
	for {

	}

}

func (RM *Server) ElectLeader() {
	log.Printf("Leader election started by assigning the remaining replica as Primary Replica")
	port := int32(5002)
	time.Sleep(10 * time.Second)
	RM.isPrimary = true
	RM.primary = RM.peers[port]

	log.Printf("New Primary Replica has port %v ", port)
}

func (RM *Server) GetHeartBeat(ctx context.Context, Heartbeat *distSystem.BeatRequest) (*distSystem.BeatAck, error) {
	return &distSystem.BeatAck{Port: fmt.Sprint(RM.id)}, nil
}

func (RM *Server) Add(ctx context.Context, AddRequest *distSystem.AddRequest) (*distSystem.AddResponse, error) {
	<-RM.lock
	word := AddRequest.GetWord()
	definition := AddRequest.GetDefinition()
	time.Sleep(3 * time.Second)
	if RM.isPrimary == true {
		//Primary Replica added the value to its own dictionary
		RM.dictionary[word] = definition
		log.Printf("Primary Replica %v: Added definition to dictionary", RM.id)
		//Primary Replica distributes the add request to the replica
		for id, server := range RM.peers {
			_, err := server.Add(RM.ctx, AddRequest)
			if err != nil {
				log.Printf("Primary Replica %v: Adding definition to Replica has failed, Replica on port %v died", RM.id, id)
				delete(RM.peers, id)
			}
		}
		RM.lock <- true
		return &distSystem.AddResponse{Ack: true}, nil

	} else {
		//Replica adds/updates the word and definition
		RM.dictionary[word] = definition
		log.Printf("Backup Replica %v: Added new definition", RM.id)
		RM.lock <- true
		return &distSystem.AddResponse{Ack: true}, nil

	}
	RM.lock <- true
	return &distSystem.AddResponse{Ack: false}, nil
}

func (RM *Server) Read(ctx context.Context, ReadRequest *distSystem.ReadRequest) (*distSystem.ReadResponse, error) {
	<-RM.lock
	definition := RM.dictionary[ReadRequest.GetWord()]
	if definition == "" {
		definition = "Definition to requested word has not been added yet."
	} else {
		log.Printf("Primary Replica %v: Reads definition '%s' to word '%s'", RM.id, definition, ReadRequest.GetWord())
	}
	RM.lock <- true
	return &distSystem.ReadResponse{Definition: definition}, nil
}

func setLogRMServer() *os.File {
	f, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	return f
}
