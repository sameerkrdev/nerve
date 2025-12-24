import {
  OrderSideEnumStringMap,
  OrderStatusEnumStringMap,
  OrderTypeEnumStringMap,
} from "@/constants";
import type { Logger } from "@repo/logger";
import { type OrderRepository } from "@repo/prisma";
import type { OrderSide, OrderStatus, OrderType } from "@repo/prisma/";
import type { CreateOrderRequest } from "@repo/proto-defs/ts/api/order_service";
import type { OrderStatus as Status } from "@repo/proto-defs/ts/common/order_types";

export class OrderServerController {
  constructor(
    private readonly logger: Logger,
    private orderRepo?: OrderRepository,
  ) {}

  async createOrder(data: CreateOrderRequest & { id: string; status: Status; eventType: string }) {
    try {
      if (!this.orderRepo) {
        this.logger.warn("OrderRepository not provided, skipping order persistence");
        return;
      }

      const newOrder = await this.orderRepo.create({
        id: data.id,
        side: OrderSideEnumStringMap[data.side] as OrderSide,
        type: OrderTypeEnumStringMap[data.type] as OrderType,
        status: OrderStatusEnumStringMap[data.status] as OrderStatus,
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
