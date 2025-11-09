import { KafkaClient } from "@repo/kakfa-client";
import type { Logger } from "@repo/logger";
import { logger } from "@repo/logger";
import type { CreateOrderRequest, Status } from "@repo/proto-defs/ts/order_service";
import { OrderServerController } from "@/controllers/order.controller";
import { OrderRepository } from "@repo/prisma";

class KafkaConsumer {
  private kafkaClient: KafkaClient;
  private orderController: OrderServerController;
  private logger: Logger = logger;
  private orderRepo: OrderRepository;

  constructor() {
    this.kafkaClient = new KafkaClient("order-service", [
      "localhost:19092",
      "localhost:19093",
      "localhost:19094",
    ]);
    this.orderRepo = new OrderRepository();
    this.orderController = new OrderServerController(this.logger, this.kafkaClient, this.orderRepo);
  }

  startConsuming(): void {
    this.kafkaClient.subscribe<
      CreateOrderRequest & { id: string; status: Status; eventType: string }
    >("order-service", "orders", async (data) => {
      this.logger.log("Consumed message from 'orders' topic:", data);

      switch (data.eventType) {
        case "create":
          this.logger.log(`Processing order creation for order ID:`, data.id);

          await this.orderController.createOrder(data);

          break;
        default:
          this.logger.warn(`Unknown event type: ${data.eventType}`);
      }
    });
  }
}

export default KafkaConsumer;
