import {
  KAFKA_CLIENT_ID,
  KAFKA_CONSUMER_GROUP_ID,
  KAFKA_TOPICS,
  KafkaClient,
} from "@repo/kakfa-client";
import type { Logger } from "@repo/logger";
import { logger } from "@repo/logger";
import type { CreateOrderRequest, Status } from "@repo/proto-defs/ts/order_service";
import { OrderServerController } from "@/controllers/order.controller";
import { OrderRepository } from "@repo/prisma";
import env from "@/config/dotenv";

class KafkaConsumer {
  private kafkaClient: KafkaClient;
  private orderController: OrderServerController;
  private logger: Logger = logger;
  private orderRepo: OrderRepository;

  constructor() {
    this.kafkaClient = new KafkaClient(
      KAFKA_CLIENT_ID.ORDER_CONSUMER_SERVICE,
      env.KAFKA_BROKERS.split(","),
    );
    this.orderRepo = new OrderRepository();
    this.orderController = new OrderServerController(this.logger, this.kafkaClient, this.orderRepo);
  }

  startConsuming(): void {
    this.kafkaClient.subscribe<
      CreateOrderRequest & { id: string; status: Status; eventType: string }
    >(
      KAFKA_CONSUMER_GROUP_ID.ORDER_CONSUMER_SERVICE_1,
      KAFKA_TOPICS.ORDERS,
      async (data, topic, partition) => {
        this.logger.info("Consumed message from 'orders' topic:", { data, topic, partition });

        switch (data.eventType) {
          case "create":
            this.logger.info(`Processing order creation for order ID:`, data.id);

            await this.orderController.createOrder(data);

            break;
          default:
            this.logger.warn(`Unknown event type: ${data.eventType}`);
        }
      },
    );
  }
}

export default KafkaConsumer;
