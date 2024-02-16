package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: grpchealthping (grpc address)")
		os.Exit(1)
	}

	address := os.Args[1]
	slog.Info("dialing with insecure credentials ...", "address", address)

	ctx := context.Background()
	client, err := grpc.DialContext(ctx, address,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	healthClient := healthpb.NewHealthClient(client)

	slog.Info("sending health check with empty service name ...")
	resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		panic(err)
	}
	slog.Info("got health check response", "status", resp.GetStatus())
}
