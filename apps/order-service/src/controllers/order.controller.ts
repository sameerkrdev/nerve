import * as grpc from "@grpc/grpc-js";
import type { Logger } from "@repo/logger";
import type {
  MatchingEngineClient,
  PlaceOrderRequest,
  PlaceOrderResponse,
  CancelOrderRequest as ServerCancelOrderRequest,
  CancelOrderResponse as ServerCancelOrderResponse,
  ModifyOrderRequest as ServerModifyOrderRequest,
  ModifyOrderResponse as ServerModifyOrderResponse,
} from "@repo/proto-defs/ts/engine/order_matching";
import type {
  CreateOrderRequest,
  CreateOrderResponse,
  CancelOrderRequest,
  CancelOrderResponse,
  ModifyOrderRequest,
  ModifyOrderResponse,
} from "@repo/proto-defs/ts/api/order_service";

export class OrderServerController {
  constructor(
    private readonly logger: Logger,
    private matchingEngineClient: MatchingEngineClient,
  ) {}

  async placeOrder(
    call: grpc.ServerUnaryCall<CreateOrderRequest, CreateOrderResponse>,
    callback: grpc.sendUnaryData<CreateOrderResponse>,
  ): Promise<void> {
    const order = call.request;
    const orderId = crypto.randomUUID();

    this.logger.info("Received order request", { order, orderId });

    try {
      const request: PlaceOrderRequest = {
        clientOrderId: orderId,
        symbol: order.symbol,
        price: order.price,
        quantity: order.quantity,
        side: order.side,
        type: order.type,
        userId: order.userId,
        clientTimestamp: order.clientTimestamp,
        gatewayTimestamp: order.gatewayTimestamp,
      };

      const response = await new Promise<PlaceOrderResponse>((resolve, reject) => {
        this.matchingEngineClient.placeOrder(
          request,
          (err: grpc.ServiceError | null, res: PlaceOrderResponse) => {
            if (err) {
              return reject(err);
            }
            resolve(res);
          },
        );
      });

      this.logger.info("Order placed successfully", {
        orderId,
        engineStatus: response.status,
      });

      callback(null, {
        ...order,
        ...response,
      });
    } catch (error) {
      const err = error instanceof Error ? error : new Error(String(error));

      this.logger.error("Failed to place order", {
        orderId,
        message: err.message,
        stack: err.stack,
      });

      callback(
        {
          code: grpc.status.INTERNAL,
          message: err.message,
          name: "CreateOrderError",
        } as grpc.ServiceError,
        null,
      );
    }
  }

  async cancelOrder(
    call: grpc.ServerUnaryCall<CancelOrderRequest, CancelOrderResponse>,
    callback: grpc.sendUnaryData<CancelOrderResponse>,
  ): Promise<void> {
    const body = call.request;

    const requestBody = {
      id: body.id,
      userId: body.userId,
      symbol: body.symbol,
    } as ServerCancelOrderRequest;

    try {
      const response = await new Promise<ServerCancelOrderResponse>((resolve, reject) => {
        this.matchingEngineClient.cancelOrder(
          requestBody,
          (err: grpc.ServiceError | null, res: ServerCancelOrderResponse) => {
            if (err) {
              return reject(err);
            }
            resolve(res);
          },
        );
      });

      this.logger.info("Order Cancelled successfully", { ...response });

      callback(null, { ...response });
    } catch (error) {
      const err = error instanceof Error ? error : new Error(String(error));

      this.logger.error("Failed to cancel order", {
        orderId: requestBody.id,
        userId: requestBody.userId,
        message: err.message,
        stack: err.stack,
      });

      callback(
        {
          code: grpc.status.INTERNAL,
          message: err.message,
          name: "CancelOrderError",
        } as grpc.ServiceError,
        null,
      );
    }
  }

  async modifyOrder(
    call: grpc.ServerUnaryCall<ModifyOrderRequest, ModifyOrderResponse>,
    callback: grpc.sendUnaryData<ModifyOrderResponse>,
  ): Promise<void> {
    const { orderId, userId, symbol, newPrice, newQuantity } = call.request;
    const clientModifyId = crypto.randomUUID();

    const requestBody = {
      orderId,
      userId,
      symbol,
      newPrice,
      newQuantity,
      clientModifyId,
    } as ServerModifyOrderRequest;

    try {
      const response = await new Promise<ServerModifyOrderResponse>((resolve, reject) => {
        this.matchingEngineClient.modifyOrder(
          requestBody,
          (err: grpc.ServiceError | null, res: ServerModifyOrderResponse) => {
            if (err) {
              return reject(err);
            }
            resolve(res);
          },
        );
      });

      this.logger.info("Order modified successfully", {
        ...response,
        engineStatus: response.status,
      });

      callback(null, { ...response });
    } catch (error) {
      const err = error instanceof Error ? error : new Error(String(error));

      this.logger.error("Failed to modify order", {
        orderId: requestBody.orderId,
        userId: requestBody.userId,
        message: err.message,
        stack: err.stack,
      });

      callback(
        {
          code: grpc.status.INTERNAL,
          message: err.message,
          name: "ModifyOrderError",
        } as grpc.ServiceError,
        null,
      );
    }
  }
}
