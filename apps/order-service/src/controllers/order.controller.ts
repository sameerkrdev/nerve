import {
  OrderSideEnumStringMap,
  OrderStatusEnumStringMap,
  OrderTypeEnumStringMap,
} from "@/constants";
import * as grpc from "@grpc/grpc-js";
import type { KafkaClient } from "@repo/kakfa-client";
import type { Logger } from "@repo/logger";
import { type OrderRepository } from "@repo/prisma";
import type { OrderSide, OrderStatus, OrderType } from "@repo/prisma/";
import type { CreateOrderRequest, CreateOrderResponse } from "@repo/proto-defs/ts/order_service";
import { Status } from "@repo/proto-defs/ts/order_service";

export class OrderServerController {
  constructor(
    private readonly logger: Logger,
    private kafkaClient: KafkaClient,
    private orderRepo?: OrderRepository,
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
