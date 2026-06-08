import env from "./config/dotenv";
import { logger } from "@repo/logger";
import { AuthGrpcServer } from "./grpc/server";

const server = new AuthGrpcServer(logger, env.AUTH_SERVICE_GRPC_URL);
server.initialize();
server.start();

process.on("SIGINT", async () => {
  logger.info("Shutting down auth-service...");
  await server.shutdown();
  process.exit(0);
});

process.on("SIGTERM", async () => {
  logger.info("Shutting down auth-service...");
  await server.shutdown();
  process.exit(0);
});
