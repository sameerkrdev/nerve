import type { Logger } from "@repo/logger";
import type {
  MatchingEngineClient,
  PlaceOrderRequest,
  PlaceOrderResponse,
} from "@repo/proto-defs/ts/order_matching";

export class OrderGrpcClient {
  constructor(
    private readonly logger: Logger,
    private readonly client: MatchingEngineClient,
  ) {}

  async createOrder(request: PlaceOrderRequest): Promise<PlaceOrderResponse> {
    return new Promise((resolve, reject) => {
      this.client.placeOrder(request, (err, response) => {
        if (err) {
          this.logger.error("gRPC client error", { error: err });
          reject(err);
          return;
        }

        if (!response) {
          this.logger.error("Empty response from gRPC server");
          reject(new Error("Empty response from gRPC server"));
          return;
        }

        this.logger.info("Order processed via gRPC", { response });
        resolve(response);
      });
    });
  }
}
