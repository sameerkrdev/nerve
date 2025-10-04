import env from "@/config/dotenv";
import { logger } from "@repo/logger";
import { clickHouseManager } from "@repo/clickhouse";
import app from "@/app";

async function initializeDatabase() {
  try {
    await clickHouseManager.initialize({
      url: env.CLICKHOUSE_URL,
      username: env.CLICKHOUSE_USER,
      password: env.CLICKHOUSE_PASSWORD,
      database: env.CLICKHOUSE_DB,
    });
    logger.info("ClickHouse client initialized successfully");
  } catch (error) {
    logger.error("Failed to initialize ClickHouse client", error);
    process.exit(1); // Exit if database connection fails
  }
}

// Handle graceful shutdown
process.on("SIGINT", async () => {
  logger.info("Shutting down server...");
  await clickHouseManager.close();
  process.exit(0);
});

process.on("SIGTERM", async () => {
  logger.info("Shutting down server...");
  await clickHouseManager.close();
  process.exit(0);
});

const startServer = async () => {
  const PORT = env.PORT;

  try {
    await initializeDatabase();

    app.listen(PORT, () => {
      logger.info(`Server is running on port ${PORT}...`);
    });
  } catch (error) {
    if (error instanceof Error) {
      logger.error(error.message);
      setTimeout(() => {
        process.exit(1);
      }, 1000);
    }
  }
};

void startServer();
