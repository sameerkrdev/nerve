// import { logger } from './../../../packages/logger/src/index';
import express from "express";
import env from "@/config/dotenv";
import { logger } from "@repo/logger";

const app = express();

app.get("/", (req, res) => {
  res.json({ messasge: "Hello" });
});

app.listen(env.PORT, () => {
  logger.info(`Server is running on port: ${env.PORT}`);
});
