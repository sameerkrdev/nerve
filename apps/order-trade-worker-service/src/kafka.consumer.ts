import {
  KAFKA_CLIENT_ID,
  KAFKA_CONSUMER_GROUP_ID,
  KAFKA_TOPICS,
  KafkaClient,
} from "@repo/kafka-client";
import type { Logger } from "@repo/logger";
import { logger } from "@repo/logger";
import { OrderServerController } from "@/controllers/order.controller";
import type { OrderSide, OrderStatus, OrderType } from "@repo/prisma";
import { OrderRepository } from "@repo/prisma";
import env from "@/config/dotenv";
import { EngineEvent, OrderStatusEvent } from "@repo/proto-defs/ts/engine/order_matching";
import {
  EventType,
  orderStatusFromJSON,
  orderTypeFromJSON,
  sideFromJSON,
} from "@repo/proto-defs/ts/common/order_types";

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
    this.orderController = new OrderServerController(this.logger, this.orderRepo);
  }

  async startConsuming(): Promise<void> {
    this.kafkaClient.subscribe<Buffer, { sequence: string }>(
      KAFKA_CONSUMER_GROUP_ID.ORDER_CONSUMER_SERVICE_1,
      KAFKA_TOPICS.ORDERS,
      async (event, topic, partition, headers) => {
        this.logger.info("Consumed message from 'orders' topic:", { topic, partition });

        if (headers.sequence) {
          // TODO: idempotency
        }

        const unmarshedEvent = EngineEvent.decode(event);

        switch (unmarshedEvent.eventType) {
          case EventType.ORDER_ACCEPTED: {
            this.logger.info(
              `Processing order accept event for user: ${unmarshedEvent.userId} of symbol ${unmarshedEvent.symbol}`,
            );

            const data = OrderStatusEvent.decode(unmarshedEvent.data);

            await this.orderController.createOrder({
              id: data.orderId,
              symbol: data.symbol,
              statusMessage: data.statusMessage,
              filledQuantity: data.filledQuantity,
              cancelledQuantity: data.cancelledQuantity,
              quantity: data.quantity,
              averagePrice: data.averagePrice,
              userId: data.userId,
              price: data.price,
              remainingQuantity: data.remainingQuantity,
              executedValue: data.executedValue,
              gatewayTimestamp: data.gatewayTimestamp,
              clientTimestamp: data.clientTimestamp,
              engineTimestamp: data.engineTimestamp,
              side: sideFromJSON(data.side) as unknown as OrderSide,
              type: orderTypeFromJSON(data.type) as unknown as OrderType,
              status: orderStatusFromJSON(data.status) as unknown as OrderStatus,
            });

            break;
          }
          default:
            this.logger.warn(`Unknown event type: ${unmarshedEvent.eventType}`);
        }
      },
    );
  }

  async shutdown(): Promise<void> {
    await this.kafkaClient.disconnectConsumer();
    this.logger.info("Kafka consumer disconnected");
  }
}

export default KafkaConsumer;
