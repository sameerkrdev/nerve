import { config } from "dotenv";
import { cleanEnv, str, num } from "envalid";
import path from "path";

config({ path: path.join(__dirname, "../../.env") });

export const env = cleanEnv(process.env, {
  AUTH_SERVICE_GRPC_URL: str({ default: "0.0.0.0:50054" }),
  NODE_ENV: str({ choices: ["development", "test", "production"], default: "development" }),
  REDIS_URL: str(),
  JWT_PRIVATE_KEY: str(),
  JWT_PUBLIC_KEY: str(),
  ACCESS_TOKEN_EXPIRY: num({ default: 900 }),
  REFRESH_TOKEN_EXPIRY: num({ default: 604800 }),
});

export default env;
