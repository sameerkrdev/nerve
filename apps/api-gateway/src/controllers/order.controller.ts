import { type Logger } from "@repo/logger";
import type { Response, NextFunction } from "express";
import type { CancelOrderRequest, CreateOrderRequest } from "@/types";
import {
  type CreateOrderResponse,
  type CreateOrderRequest as GrpcCreateOrderRequest,
  type OrderServiceClient,
  type CancelOrderResponse,
} from "@repo/proto-defs/ts/api/order_service";

import type grpc from "@grpc/grpc-js";
import { Side, OrderType as Type } from "@repo/proto-defs/ts/common/order_types";

export class OrderController {
  constructor(
    private logger: Logger,
    private grpcEngine: OrderServiceClient,
  ) {}

  createOrder = (req: CreateOrderRequest, res: Response, next: NextFunction) => {
    const { symbol, price, quantity, side, type } = req.body;
    const userId = "d8036c81-a1d7-45de-b4d8-e3847bfadd3b"; // TODO: replace with authenticated userId --> req.userId

    const grpcRequest: GrpcCreateOrderRequest = {
      symbol,
      price: price!, // TODO: price is optional in proto, but required for limit orders
      quantity,
      side: Side[side as keyof typeof Side],
      type: Type[type as keyof typeof Type],
      userId,
      clientTimestamp: new Date(),
      gatewayTimestamp: new Date(),
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

  cancelOrder = (req: CancelOrderRequest, res: Response, next: NextFunction) => {
    const id = req.params.id;
    const userId = "d8036c81-a1d7-45de-b4d8-e3847bfadd3b"; // TODO: replace with authenticated userId --> req.userId

    const requestBody = {
      id: id,
      userId: userId,
      symbol: req.body.symbol,
    };

    this.grpcEngine.cancelOrder(
      requestBody,
      (err: grpc.ServiceError | null, response: CancelOrderResponse) => {
        if (err) return next(err);

        this.logger.info("Order Cancelled", { response });
        res
          .status(200)
          .json({ status: "success", message: "Order Cancelled Successfully", data: response });
      },
    );
  };
}
