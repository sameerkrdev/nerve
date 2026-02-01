import { config } from "dotenv";
import { cleanEnv, str } from "envalid";
import path from "path";

config({
  path: path.join(__dirname, `../../.env.${process.env.NODE_ENV || "development"}`),
});

export const env = cleanEnv(process.env, {
  ORDER_SERVICE_GRPC_URL: str(),

  NODE_ENV: str({ choices: ["development", "test", "production"] }),

  KAFKA_BROKERS: str(),
});

export default env;
