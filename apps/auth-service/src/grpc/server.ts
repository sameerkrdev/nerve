import * as grpc from "@grpc/grpc-js";
import { AuthServiceService } from "@repo/proto-defs/ts/auth/v1/auth_service";
import { type Logger } from "@repo/logger";
import { authHandlers } from "./handlers";

export class AuthGrpcServer {
  private server: grpc.Server;

  constructor(
    private readonly logger: Logger,
    private readonly address: string,
  ) {
    this.server = new grpc.Server();
  }

  initialize(): void {
    this.server.addService(AuthServiceService, authHandlers);
  }

  start(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.server.bindAsync(this.address, grpc.ServerCredentials.createInsecure(), (err, port) => {
        if (err) {
          this.logger.error("Failed to start auth gRPC server", { error: err });
          reject(err);
          return;
        }
        this.logger.info(`auth gRPC server running on port ${port}`);
        this.server.start();
        resolve();
      });
    });
  }

  async shutdown(): Promise<void> {
    return new Promise((resolve) => {
      this.server.tryShutdown(() => {
        this.logger.info("auth gRPC server shut down");
        resolve();
      });
    });
  }
}
