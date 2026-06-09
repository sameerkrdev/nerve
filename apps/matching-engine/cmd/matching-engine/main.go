package main

import (
	"log"
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"

	"github.com/joho/godotenv"
	internal "github.com/sameerkrdev/nerve/apps/matching-engine/internal"

	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
)

func main() {
	godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		if err := internal.InitRedis(redisURL); err != nil {
			slog.Warn("redis init failed — order/depth events will not be published", "err", err)
		}
	} else {
		slog.Warn("REDIS_URL not set — order/depth events will not be published")
	}

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatalf("Failed to serve: %v", "PORT is required")
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to start grpc server on port %s with error: %v", port, err)
	}

	var ops []grpc.ServerOption
	grpcServer := grpc.NewServer(ops...)

	matchingEngineServer := &internal.Server{}

	pb.RegisterMatchingEngineServer(grpcServer, matchingEngineServer)

	symbols := []internal.Symbol{
		{Name: "BTCUSD", StartingPrice: 90_000, MaxWalFileSize: 67_108_864, WalDir: "wal", WalSyncInterval: 400, WalShouldFsync: true, KafkaBatchSize: 300, KafkaEmitMM: 2000},
		{Name: "SOLUSD", StartingPrice: 150, MaxWalFileSize: 67_108_864, WalDir: "wal", WalSyncInterval: 400, WalShouldFsync: true, KafkaBatchSize: 300, KafkaEmitMM: 2000},
		{Name: "ETHUSD", StartingPrice: 3_510, MaxWalFileSize: 67_108_864, WalDir: "wal", WalSyncInterval: 400, WalShouldFsync: true, KafkaBatchSize: 300, KafkaEmitMM: 2000},
	}

	internal.StartActors(symbols)

	log.Printf("gRPC server listening at %v", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
