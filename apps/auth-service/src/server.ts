import env from "./config/dotenv";
import { logger } from "@repo/logger";
import { AuthGrpcServer } from "./grpc/server";

const server = new AuthGrpcServer(logger, env.AUTH_SERVICE_GRPC_URL);
server.initialize();
void server.start();

process.on("SIGINT", async () => {
  logger.info("AUTH SIGINT RECEIVED");
  logger.info("Shutting down auth-service...");
  await server.shutdown();
  process.exit(0);
});

process.on("SIGTERM", async () => {
  logger.info("AUTH SIGTERM RECEIVED");
  logger.info("Shutting down auth-service...");
  await server.shutdown();
  process.exit(0);
});
