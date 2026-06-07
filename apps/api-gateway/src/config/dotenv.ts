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
});

export default env;
