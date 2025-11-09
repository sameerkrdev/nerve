import { logger } from "@repo/logger";
import { GrpcServer } from "@/grpc.server";
import KafkaConsumer from "@/kafka.consumer";

const server = new GrpcServer(logger, "0.0.0.0:50051");
server.initialize();
server.start();

const kafkaConsumerInstance = new KafkaConsumer();
kafkaConsumerInstance.startConsuming();

// Graceful shutdown
process.on("SIGINT", async () => {
  logger.info("Received SIGINT, shutting down gracefully");
  await server.shutdown();
  process.exit(0);
});

process.on("SIGTERM", async () => {
  logger.info("Received SIGTERM, shutting down gracefully");
  await server.shutdown();
  process.exit(0);
});
