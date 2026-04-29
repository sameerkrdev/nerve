import { logger } from "@repo/logger";
import KafkaConsumer from "@/kafka.consumer";

const kafkaConsumerInstance = new KafkaConsumer();
kafkaConsumerInstance.startConsuming();

// Graceful shutdown
process.on("SIGINT", async () => {
  logger.info("Received SIGINT, shutting down gracefully");
  await kafkaConsumerInstance.shutdown();
  process.exit(0);
});

process.on("SIGTERM", async () => {
  logger.info("Received SIGTERM, shutting down gracefully");
  await kafkaConsumerInstance.shutdown();
  process.exit(0);
});
