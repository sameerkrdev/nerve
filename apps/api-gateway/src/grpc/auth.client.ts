import * as grpc from "@grpc/grpc-js";
import { AuthServiceClient } from "@repo/proto-defs/ts/auth/v1/auth_service";
import env from "@/config/dotenv";
import { logger } from "@repo/logger";

const credentials =
  process.env.NODE_ENV === "production"
    ? grpc.credentials.createSsl()
    : grpc.credentials.createInsecure();

const authClient = new AuthServiceClient(env.AUTH_SERVICE_GRPC_URL, credentials);
logger.info(`Connected to Auth gRPC service at ${env.AUTH_SERVICE_GRPC_URL}`);

export { authClient };
