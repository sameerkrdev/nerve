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
      price: price!, // TODO: price is optional in proto, but required for limit orders
      quantity,
      side: Side[side as keyof typeof Side],
      type: Type[type as keyof typeof Type],
      userId: "d8036c81-a1d7-45de-b4d8-e3847bfadd3b",
      clientTimestamp: new Date(),
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
