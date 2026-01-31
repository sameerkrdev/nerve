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
import { TradeRepository } from "@repo/prisma";
import { OrderRepository } from "@repo/prisma";
import env from "@/config/dotenv";
import {
  EngineEvent,
  OrderReducedEvent,
  OrderStatusEvent,
  TradeEvent,
} from "@repo/proto-defs/ts/engine/order_matching";
import {
  EventType,
  orderStatusToJSON,
  orderTypeToJSON,
  sideToJSON,
} from "@repo/proto-defs/ts/common/order_types";
import { TradeServerController } from "./controllers/trade.controller";

class KafkaConsumer {
  private kafkaClient: KafkaClient;
  private orderController: OrderServerController;
  private tradeController: TradeServerController;
  private logger: Logger = logger;
  private orderRepo: OrderRepository;
  private tradeRepo: TradeRepository;

  constructor() {
    this.kafkaClient = new KafkaClient(
      KAFKA_CLIENT_ID.ORDER_CONSUMER_SERVICE,
      env.KAFKA_BROKERS.split(","),
    );
    this.orderRepo = new OrderRepository();
    this.tradeRepo = new TradeRepository();
    this.orderController = new OrderServerController(this.logger, this.orderRepo);
    this.tradeController = new TradeServerController(this.logger, this.tradeRepo);
  }

  async startConsuming(): Promise<void> {
    this.kafkaClient.subscribe<Buffer, { sequence: string }>(
      KAFKA_CONSUMER_GROUP_ID.ORDER_CONSUMER_SERVICE_1,
      KAFKA_TOPICS.ENGINE_ENVENTS,
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
              gatewayTimestamp: data.gatewayTimestamp!,
              clientTimestamp: data.clientTimestamp!,
              engineTimestamp: data.engineTimestamp!,
              side: sideToJSON(data.side) as unknown as OrderSide,
              type: orderTypeToJSON(data.type) as unknown as OrderType,
              status: orderStatusToJSON(data.status) as unknown as OrderStatus,
            });

            break;
          }

          case EventType.TRADE_EXECUTED: {
            this.logger.info(
              `Processing trade excuted event for user: ${unmarshedEvent.userId} of symbol ${unmarshedEvent.symbol}`,
            );

            const data = TradeEvent.decode(unmarshedEvent.data);

            // create trade
            await this.tradeController.createTrade({
              id: data.tradeId,
              symbol: data.symbol,
              price: data.price,
              quantity: data.quantity,
              tradeSequence: data.tradeSequence,
              isBuyerMaker: data.isBuyerMaker,

              sellerId: data.sellerId,
              buyerId: data.buyerId,

              sellOrderId: data.sellOrderId,
              buyOrderId: data.buyOrderId,
              timestamp: data.timestamp!,
            });

            // update maker and taker order detail
            await this.orderController.updateOrderForTradeExcute({
              id: data.buyOrderId,
              price: data.price,
              quantity: data.quantity,
            });
            await this.orderController.updateOrderForTradeExcute({
              id: data.sellOrderId,
              price: data.price,
              quantity: data.quantity,
            });

            break;
          }

          case EventType.ORDER_REDUCED: {
            this.logger.info(
              `Processing order reduced event for user: ${unmarshedEvent.userId} of symbol ${unmarshedEvent.symbol}`,
            );

            const data = OrderReducedEvent.decode(unmarshedEvent.data);

            if (!data.order) {
              this.logger.error("Order id not found");
              break;
            }

            // update maker and taker order detail
            await this.orderController.updateOrderForQuantityReduced({
              id: data.order.orderId,
              newCancelledQuantity: data.newCancelledQuantity,
              newRemainingQuantiy: data.newRemainingQuantity,
            });
            break;
          }

          case EventType.ORDER_CANCELLED: {
            this.logger.info(
              `Processing order cancelled event for user: ${unmarshedEvent.userId} of symbol ${unmarshedEvent.symbol}`,
            );

            const data = OrderStatusEvent.decode(unmarshedEvent.data);

            // update maker and taker order detail
            await this.orderController.updateOrderForCancelled({
              id: data.orderId,
              cancelledQuantity: data.cancelledQuantity,
              remainingQuantiy: data.remainingQuantity,
            });
            break;
          }

          case EventType.ORDER_REJECTED: {
            this.logger.info(
              `Processing order rejected event for user: ${unmarshedEvent.userId} of symbol ${unmarshedEvent.symbol}`,
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
              gatewayTimestamp: data.gatewayTimestamp!,
              clientTimestamp: data.clientTimestamp!,
              engineTimestamp: data.engineTimestamp!,
              side: sideToJSON(data.side) as unknown as OrderSide,
              type: orderTypeToJSON(data.type) as unknown as OrderType,
              status: orderStatusToJSON(data.status) as unknown as OrderStatus,
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
