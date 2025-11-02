import { type Logger } from "@repo/logger";
import type { Response, NextFunction } from "express";
import type { CreateOrderRequest } from "@/types";
import {
  Side,
  Type,
  type CreateOrderResponse,
  type CreateOrderRequest as GrpcCreateOrderRequest,
  type OrderServiceClient,
} from "@repo/proto-defs/ts/order_service";
import type grpc from "@grpc/grpc-js";

export class OrderController {
  constructor(
    private logger: Logger,
    private grpcEngine: OrderServiceClient,
  ) {}

  createOrder = (req: CreateOrderRequest, res: Response, next: NextFunction) => {
    const { symbol, price, quantity, side, type } = req.body;

    const grpcRequest: GrpcCreateOrderRequest = {
      symbol,
      price,
      quantity,
      side: Side[side as keyof typeof Side],
      type: Type[type as keyof typeof Type],
      userId: "user-123",
      clientTimeline: new Date(),
    };

    this.grpcEngine.createOrder(
      grpcRequest,
      (err: grpc.ServiceError | null, response: CreateOrderResponse) => {
        if (err) return next(err);

        this.logger.info("Order placed", { response });
        res.json({ message: "Order is placed successfully", data: response });
      },
    );
  };
}
