import * as grpc from "@grpc/grpc-js";
import { OrderServiceService, type OrderServiceServer } from "@repo/proto-defs/ts/order_service";
import { type Logger } from "@repo/logger";
import { OrderServerController } from "@/controllers/order.controller";
import { KafkaClient } from "@repo/kakfa-client";

export class GrpcServer {
  private server: grpc.Server;
  private kafkaCient: KafkaClient;

  constructor(
    private readonly logger: Logger,
    private readonly address: string,
  ) {
    this.server = new grpc.Server();
    this.kafkaCient = new KafkaClient("order-producer-service", [
      "localhost:19092",
      "localhost:19093",
      "localhost:19094",
    ]);
  }

  initialize(): void {
    // Initialize dependencies
    const orderController = new OrderServerController(this.logger, this.kafkaCient);

    // Define service implementation
    const orderServiceImpl: OrderServiceServer = {
      createOrder: orderController.placeOrder.bind(orderController),
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
