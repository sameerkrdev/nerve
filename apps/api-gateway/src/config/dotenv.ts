import { config } from "dotenv";
import { cleanEnv, port, str } from "envalid";
import path from "path";

config({
  path: path.join(__dirname, `../../.env`),
});

export const env = cleanEnv(process.env, {
  PORT: port(),
  NODE_ENV: str({ choices: ["development", "test", "production"] }),
  ORDER_SERVICE_GRPC_URL: str(),
  JWT_PUBLIC_KEY: str(),
  REDIS_URL: str(),
  AUTH_SERVICE_GRPC_URL: str({ default: "localhost:50054" }),
});

export default env;
