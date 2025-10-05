import { config } from "dotenv";
import { cleanEnv, port, str } from "envalid";
import path from "node:path";

config({
  path: path.join(__dirname, `../../.env.${process.env.NODE_ENV || "development"}`),
});

const env = cleanEnv(process.env, {
  PORT: port(),
  NODE_ENV: str({ choices: ["development", "production", "test"] }),
});

export default env;
