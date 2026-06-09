import Redis from "ioredis";
import env from "./config/dotenv";
import { logger } from "@repo/logger";

export const redisClient = new Redis(env.REDIS_URL);

redisClient.on("connect", () => {
  logger.info("Redis connected");
});

redisClient.on("error", (err) => {
  logger.error("Redis error", {
    message: "Redis error",
    error: err,
  });
});
