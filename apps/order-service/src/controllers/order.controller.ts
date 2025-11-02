import type { OrderService } from "@/services/order.service";
import * as grpc from "@grpc/grpc-js";
import type { Logger } from "@repo/logger";
import type { CreateOrderRequest, CreateOrderResponse } from "@repo/proto-defs/ts/order_service";

export class OrderServerController {
  constructor(
    private readonly logger: Logger,
    private readonly orderService: OrderService,
  ) {}

  createOrder = async (
    call: grpc.ServerUnaryCall<CreateOrderRequest, CreateOrderResponse>,
    callback: grpc.sendUnaryData<CreateOrderResponse>,
  ): Promise<void> => {
    const order = call.request;
    this.logger.info("Received order request", { order });

    try {
      const response = await this.orderService.createOrder(order);
      callback(null, response);
    } catch (error) {
      const err = error instanceof Error ? error : new Error(String(error));
      this.logger.error("Failed to create order", { stack: err.stack });
      callback(
        {
          code: grpc.status.INTERNAL,
          message: err.message || "Failed to create order",
          name: "CreateOrderError",
        },
        null,
      );
    }
  };
}
