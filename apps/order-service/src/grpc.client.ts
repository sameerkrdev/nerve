import env from "@/config/dotenv";
import * as grpc from "@grpc/grpc-js";
import { logger } from "@repo/logger";
import { MatchingEngineClient } from "@repo/proto-defs/ts/engine/order_matching";

export class MatchingEngineGrpcClient {
  constructor() {}

  start(): MatchingEngineClient {
    const matchingEngineServerUrl = env.MATCHING_SERVICE_GRPC_URL;

    const credentials =
      env.NODE_ENV === "production"
        ? grpc.credentials.createSsl()
        : grpc.credentials.createInsecure();

    const client = new MatchingEngineClient(matchingEngineServerUrl, credentials);

    logger.info(`Connected to Matching Enginer gRPC server at ${matchingEngineServerUrl}`);

    return client;
  }
}
