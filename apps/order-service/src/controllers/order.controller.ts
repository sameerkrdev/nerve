import * as grpc from "@grpc/grpc-js";
import type { KafkaClient } from "@repo/kakfa-client";
import type { Logger } from "@repo/logger";
import type { CreateOrderRequest, CreateOrderResponse } from "@repo/proto-defs/ts/order_service";
import { Status } from "@repo/proto-defs/ts/order_service";

export class OrderServerController {
  constructor(
    private readonly logger: Logger,
    private kafkaClient: KafkaClient,
  ) {}

  async placeOrder(
    call: grpc.ServerUnaryCall<CreateOrderRequest, CreateOrderResponse>,
    callback: grpc.sendUnaryData<CreateOrderResponse>,
  ): Promise<void> {
    const order = call.request;
    this.logger.info("Received order request", { order });

    try {
      const orderId = crypto.randomUUID();

      // Send this to kafka for order persistence
      this.kafkaClient.sendMessage<
        CreateOrderRequest & { id: string; status: Status; eventType: string }
      >("orders", { id: orderId, status: Status.PENDING, eventType: "create", ...order });

      // Place the order to matching engine

      const response = {
        id: orderId,
        ...order,
        status: Status.PENDING,
        reason: "Order placed successfully",
      };

      this.logger.info("Order placed successfully");
      callback(null, response);
    } catch (error) {
      const err = error instanceof Error ? error : new Error(String(error));
      this.logger.error("Failed to create order", {
        message: err.message,
        stack: err.stack,
      });

      callback(
        {
          code: grpc.status.INTERNAL,
          message: err.message || "Failed to create order",
          name: "CreateOrderError",
        } as grpc.ServiceError,
        null,
      );
    }
  }
}
