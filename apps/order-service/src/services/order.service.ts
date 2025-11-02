import type { Logger } from "@repo/logger";
import {
  Status,
  type CreateOrderRequest,
  type CreateOrderResponse,
} from "@repo/proto-defs/ts/order_service";

export class OrderService {
  constructor(private readonly logger: Logger) {}

  async createOrder(request: CreateOrderRequest): Promise<CreateOrderResponse> {
    // Generate order uuid
    const orderId = crypto.randomUUID();

    // Send this to kafka for order persistence

    // Place the order to matching engine

    this.logger.info("Order created successfully");

    return {
      id: orderId,
      ...request,
      status: Status.PENDING,
      reason: "Order created successfully",
    };
  }
}
