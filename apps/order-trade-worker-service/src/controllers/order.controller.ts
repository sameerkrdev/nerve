import type { Logger } from "@repo/logger";
import { type OrderRepository } from "@repo/prisma";
import type { OrderSide, OrderStatus, OrderType } from "@repo/prisma/";

export class OrderServerController {
  constructor(
    private readonly logger: Logger,
    private orderRepo: OrderRepository,
  ) {}

  async createOrder(data: {
    id: string;
    side: OrderSide;
    type: OrderType;
    userId: string;
    status: OrderStatus;
    statusMessage: string | undefined;
    symbol: string;
    price: number;
    executedValue: number;
    quantity: number;
    averagePrice: number;
    filledQuantity: number;
    cancelledQuantity: number;
    remainingQuantity: number;
    gatewayTimestamp?: Date | undefined;
    clientTimestamp?: Date | undefined;
    engineTimestamp?: Date | undefined;
  }) {
    try {
      if (!this.orderRepo) {
        this.logger.warn("OrderRepository not provided, skipping order persistence");
        return;
      }

      const newOrder = await this.orderRepo.create({
        id: data.id,
        side: data.side,
        type: data.type,
        status: data.status,
        user: { connect: { id: data.userId } },
        symbol: data.symbol,
        price: data.price,
        quantity: data.quantity,
        remaining_quantity: data.quantity,
        time_in_force: "GTC",
        filled_quantity: 0,
      });

      this.logger.info("Order persisted successfully", { orderId: newOrder.id });
    } catch (error) {
      this.logger.error("Failed to persist order", {
        message: error instanceof Error ? error.message : String(error),
      });

      throw error;
    }
  }
}
