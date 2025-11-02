import * as grpc from "@grpc/grpc-js";
import { OrderServiceService, type OrderServiceServer } from "@repo/proto-defs/ts/order_service";
import { type Logger, logger } from "@repo/logger";
import { OrderServerController } from "@/controllers/order.controller";
import { OrderService } from "@/services/order.service";

export class GrpcServer {
  private server: grpc.Server;

  constructor(
    private readonly logger: Logger,
    private readonly address: string,
  ) {
    this.server = new grpc.Server();
  }

  initialize(): void {
    // Initialize dependencies
    const orderService = new OrderService(this.logger);
    const orderController = new OrderServerController(this.logger, orderService);

    // Define service implementation
    const orderServiceImpl: OrderServiceServer = {
      createOrder: orderController.createOrder.bind(orderController),
    };

    // Add service to server
    this.server.addService(OrderServiceService, orderServiceImpl);
  }

  start(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.server.bindAsync(this.address, grpc.ServerCredentials.createInsecure(), (err, port) => {
        if (err) {
          this.logger.error("Failed to start gRPC server", { error: err });
          reject(err);
          return;
        }

        this.logger.info(`gRPC server running at ${this.address} on port ${port}`);
        this.server.start();
        resolve();
      });
    });
  }

  async shutdown(): Promise<void> {
    return new Promise((resolve) => {
      this.server.tryShutdown(() => {
        this.logger.info("gRPC server shut down gracefully");
        resolve();
      });
    });
  }
}

const server = new GrpcServer(logger, "0.0.0.0:50051");
server.initialize();
server.start();

// Graceful shutdown
process.on("SIGINT", async () => {
  logger.info("Received SIGINT, shutting down gracefully");
  await server.shutdown();
  process.exit(0);
});

process.on("SIGTERM", async () => {
  logger.info("Received SIGTERM, shutting down gracefully");
  await server.shutdown();
  process.exit(0);
});
