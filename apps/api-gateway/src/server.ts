import env from "@/config/dotenv";
import { logger } from "@repo/logger";
import app from "@/app";

// Handle graceful shutdown
process.on("SIGINT", async () => {
  logger.info("Shutting down server...");
  process.exit(0);
});

process.on("SIGTERM", async () => {
  logger.info("Shutting down server...");
  process.exit(0);
});

const startServer = async () => {
  const PORT = env.PORT;

  try {
    app.listen(PORT, () => {
      logger.info(`Server is running on port ${PORT}...`);
    });
  } catch (error) {
    if (error instanceof Error) {
      logger.error(error.message);
      process.exit(1);
    }
  }
};

void startServer();
