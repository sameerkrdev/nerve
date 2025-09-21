import express from "express";
import env from "@/config/dotenv";
import { logger } from "@repo/logger";
import { clickHouseManager, TradeRepository } from "@repo/database";

const app = express();
app.use(express.json());

// Initialize ClickHouse connection with proper error handling
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

const tradeRepo = new TradeRepository();

app.get("/", (_, res) => {
  res.json({ message: "Hello" });
});

app.post("/", async (req, res) => {
  try {
    await tradeRepo.createTrade(req.body);
    res.status(201).json({ mesasge: "New Trade is created", success: true });
  } catch (error) {
    logger.error("Failed to create new trade", error);
    res.status(500).json({
      message: "Failed to create new trade",
      success: false,
    });
  }
});

// Start server only after database is initialized
async function startServer() {
  await initializeDatabase();

  app.listen(env.PORT, () => {
    logger.info(`Server is running on port: ${env.PORT}`);
  });
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

startServer().catch((error) => {
  logger.error("Failed to start server", error);
  process.exit(1);
});
