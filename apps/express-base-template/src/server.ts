import app from "@/app";
import env from "@/config/dotenv";
import { logger } from "@repo/logger";

process.on("SIGINT", () => {
  logger.info("Shutting down server...");
  process.exit(0);
});

process.on("SIGTERM", () => {
  logger.info("Shutting down server...");
  process.exit(0);
});

const startServer = () => {
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
startServer();
