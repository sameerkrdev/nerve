import * as grpc from "@grpc/grpc-js";
import type { KafkaClient } from "@repo/kakfa-client";
import type { Logger } from "@repo/logger";
import type {
  MatchingEngineClient,
  PlaceOrderRequest,
  PlaceOrderResponse,
} from "@repo/proto-defs/ts/engine/order_matching";
import type {
  CreateOrderRequest,
  CreateOrderResponse,
} from "@repo/proto-defs/ts/api/order_service";
import { OrderStatus as Status } from "@repo/proto-defs/ts/common/order_types";

export class OrderServerController {
  constructor(
    private readonly logger: Logger,
    private kafkaClient: KafkaClient,
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
      await this.kafkaClient.sendMessage<
        CreateOrderRequest & {
          id: string;
          status: Status;
          eventType: "create";
        }
      >("orders", {
        id: orderId,
        status: Status.PENDING,
        eventType: "create",
        ...order,
      });

      const request: PlaceOrderRequest = {
        id: orderId,
        symbol: order.symbol,
        price: order.price,
        quantity: order.quantity,
        side: order.side,
        type: order.type,
        userId: order.userId,
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
          message: "Failed to place order",
          name: "CreateOrderError",
        } as grpc.ServiceError,
        null,
      );
    }
  }
}
