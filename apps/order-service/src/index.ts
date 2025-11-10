import { logger } from "@repo/logger";
import { GrpcServer } from "@/grpc.server";
import KafkaConsumer from "@/kafka.consumer";
import env from "@/config/dotenv";

const server = new GrpcServer(logger, env.ORDER_SERVICE_GRPC_URL);
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
