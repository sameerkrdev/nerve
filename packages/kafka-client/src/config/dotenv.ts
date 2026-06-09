import { config } from "dotenv";
import { cleanEnv, str } from "envalid";
import path from "node:path";

config({
  path: path.join(__dirname, `../../.env`),
});

export const env = cleanEnv(process.env, {
  NODE_ENV: str({ choices: ["development", "test", "production"] }),
  KAFKA_BROKERS: str(),
  KAFKA_CA: str(),
  KAFKA_USERNAME: str(),
  KAFKA_PASSWORD: str(),
});

export default env;
