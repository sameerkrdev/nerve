import * as grpc from "@grpc/grpc-js";
import { AuthServiceClient } from "@repo/proto-defs/ts/auth/v1/auth_service";
import env from "@/config/dotenv";

const credentials =
  process.env.NODE_ENV === "production"
    ? grpc.credentials.createSsl()
    : grpc.credentials.createInsecure();

export const authClient = new AuthServiceClient(env.AUTH_SERVICE_GRPC_URL, credentials);
