import type { Logger } from "@repo/logger";
import { type OrderRepository } from "@repo/prisma";
import type { OrderSide, OrderType } from "@repo/prisma/";
import type { OrderStatus } from "@repo/prisma/";
import {
  orderStatusToJSON,
  OrderStatus as grpcOrderStatus,
} from "@repo/proto-defs/ts/common/order_types";

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
    gatewayTimestamp: Date;
    clientTimestamp: Date;
    engineTimestamp: Date;
  }) {
    try {
      if (!this.orderRepo) {
        this.logger.warn("OrderRepository not provided, skipping order persistence");
        return;
      }

      const newOrder = await this.orderRepo.create({
        id: data.id,
        symbol: data.symbol,
        side: data.side,
        type: data.type,
        status: data.status,
        status_message: data.statusMessage || null,

        user: { connect: { id: data.userId } },

        price: data.price,
        average_price: data.averagePrice,
        executedValue: data.executedValue,

        quantity: data.quantity,
        remaining_quantity: data.remainingQuantity,
        filled_quantity: data.filledQuantity,
        canelled_quantity: data.cancelledQuantity, // TODO: fix this typo

        gateway_timeline: data.gatewayTimestamp,
        client_timeline: data.clientTimestamp,
        engine_timeline: data.engineTimestamp,
      });

      this.logger.info("Order persisted successfully", { orderId: newOrder.id });
    } catch (error) {
      this.logger.error("Failed to persist order", {
        message: error instanceof Error ? error.message : String(error),
      });

      throw error;
    }
  }

  async updateOrderForTradeExcute(data: { id: string; quantity: number; price: number }) {
    try {
      if (!this.orderRepo) {
        this.logger.warn("OrderRepository not provided, skipping order persistence");
        return;
      }

      const order = await this.orderRepo.findById(data.id);
      if (!order) {
        this.logger.error("Order not found");
        return;
      }

      const executed_quantity = order.executedValue + data.price * data.quantity;
      const remaining_quantity = order.remaining_quantity - data.quantity;
      const filled_quantity = order.filled_quantity + data.quantity;
      const average_price = executed_quantity / filled_quantity;
      const status =
        remaining_quantity === 0
          ? orderStatusToJSON(grpcOrderStatus.FILLED)
          : orderStatusToJSON(grpcOrderStatus.PARTIAL_FILLED);

      const newOrder = await this.orderRepo.update(data.id, {
        status: status as OrderStatus,

        average_price: average_price,
        executedValue: executed_quantity,

        remaining_quantity: remaining_quantity,
        filled_quantity: filled_quantity,
      });

      this.logger.info("Order's trade excuted updated successfully", { orderId: newOrder.id });
    } catch (error) {
      this.logger.error("Failed to update order for trade excuted", {
        message: error instanceof Error ? error.message : String(error),
      });

      throw error;
    }
  }

  async updateOrderForQuantityReduced(data: {
    id: string;
    newRemainingQuantiy: number;
    newCancelledQuantity: number;
  }) {
    try {
      if (!this.orderRepo) {
        this.logger.warn("OrderRepository not provided, skipping order persistence");
        return;
      }

      const order = await this.orderRepo.findById(data.id);
      if (!order) {
        this.logger.error("Order not found");
        return;
      }

      const newOrder = await this.orderRepo.update(data.id, {
        remaining_quantity: data.newRemainingQuantiy,
        canelled_quantity: data.newCancelledQuantity,
      });

      this.logger.info("Order reduced updated successfully", { orderId: newOrder.id });
    } catch (error) {
      this.logger.error("Failed to update order for quantity reduced", {
        message: error instanceof Error ? error.message : String(error),
      });

      throw error;
    }
  }

  async updateOrderForCancelled(data: {
    id: string;
    remainingQuantiy: number;
    cancelledQuantity: number;
  }) {
    try {
      if (!this.orderRepo) {
        this.logger.warn("OrderRepository not provided, skipping order persistence");
        return;
      }

      const order = await this.orderRepo.findById(data.id);
      if (!order) {
        this.logger.error("Order cancelled not found");
        return;
      }

      const newOrder = await this.orderRepo.update(data.id, {
        remaining_quantity: data.remainingQuantiy,
        canelled_quantity: data.cancelledQuantity,
        status: "CANCELLED",
      });

      this.logger.info("Order cancelled updated successfully", { orderId: newOrder.id });
    } catch (error) {
      this.logger.error("Failed to update order for cancelled", {
        message: error instanceof Error ? error.message : String(error),
      });

      throw error;
    }
  }
}
