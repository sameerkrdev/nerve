package main

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	server "github.com/sameerkrdev/nerve/apps/matching-engine/internal"

	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
)

func main() {
	port := 50052

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		log.Fatalf("Failed to start grpc server on port %d with error: %v", port, err)
	}

	var ops []grpc.ServerOption
	grpcServer := grpc.NewServer(ops...)

	matchingEngineServer := &server.Server{}

	pb.RegisterMatchingEngineServer(grpcServer, matchingEngineServer)

	log.Printf("gRPC server listening at %v", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
