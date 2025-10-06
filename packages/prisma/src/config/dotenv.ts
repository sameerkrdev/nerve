import { config } from "dotenv";
import { cleanEnv, str } from "envalid";
import path from "node:path";

config({
  path: path.join(__dirname, `../../.env.${process.env.NODE_ENV || "development"}`),
});

export const env = cleanEnv(process.env, {
  NODE_ENV: str({ choices: ["development", "test", "production"] }),
  POSTGRSES_DATABASE_URL: str(),
});

export default env;
