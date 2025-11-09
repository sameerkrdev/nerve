import { Kafka, type Producer, type Consumer } from "kafkajs";
import type { z } from "@repo/validator";
import type { Logger } from "@repo/logger";
import { logger } from "@repo/logger";

class KafkaClient {
  private kafka: Kafka;
  private producer?: Producer;
  private log: Logger;

  constructor(clientId: string, brokers: string[]) {
    this.kafka = new Kafka({ clientId, brokers });
    this.log = logger.child({ component: `KafkaClient:${clientId}` });
  }

  async getProducer(): Promise<Producer> {
    if (!this.producer) {
      this.producer = this.kafka.producer({ idempotent: true });
      await this.producer.connect();
      this.log.info("Producer connected");
    }
    return this.producer;
  }

  async createConsumer(groupId: string): Promise<Consumer> {
    const consumer = this.kafka.consumer({ groupId });
    await consumer.connect();
    this.log.info(`Consumer (${groupId}) connected`);
    return consumer;
  }

  async sendMessage<T>(topic: string, message: T, key?: string, schema?: z.ZodTypeAny) {
    if (!topic) throw new Error("Topic is required");

    let payload = message;

    if (schema) {
      const result = schema.safeParse(message);
      if (!result.success) {
        this.log.error("Message validation failed", result.error.format());
        throw result.error;
      }
      payload = result.data as T;
    }

    const producer = await this.getProducer();
    await producer.send({
      topic,
      messages: [
        {
          key: key ?? null,
          value: JSON.stringify(payload),
        },
      ],
    });

    this.log.info(`Message sent -> ${topic}`, { key, payload });
  }

  async subscribe<T>(
    groupId: string,
    topic: string,
    handler: (message: T, topic: string, partition: number) => Promise<void>,
    schema?: z.ZodTypeAny,
  ) {
    const consumer = await this.createConsumer(groupId);
    await consumer.subscribe({ topic, fromBeginning: false });

    this.log.info(`Listening -> topic: ${topic}, group: ${groupId}`);

    await consumer.run({
      autoCommit: false,
      eachMessage: async ({ message, partition, topic }) => {
        try {
          const raw = message.value?.toString();
          if (!raw) return;

          const parsed = JSON.parse(raw);
          const payload = schema ? schema.parse(parsed) : parsed;

          await handler(payload, topic, partition);

          await consumer.commitOffsets([
            {
              topic,
              partition,
              offset: (Number(message.offset) + 1).toString(),
            },
          ]);
        } catch (error) {
          this.log.error("Message processing failed", error);
          // Optional: send to DLQ topic here
          await consumer.commitOffsets([
            {
              topic,
              partition,
              offset: (Number(message.offset) + 1).toString(),
            },
          ]);
        }
      },
    });
  }

  async disconnect() {
    if (this.producer) await this.producer.disconnect();
    this.log.info("Producer disconnected");
  }
}

export { KafkaClient, type Producer, type Consumer };
