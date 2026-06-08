import * as grpc from "@grpc/grpc-js";
import { isHttpError } from "http-errors";
import createHttpError from "http-errors";
import type { AuthServiceServer } from "@repo/proto-defs/ts/auth/v1/auth_service";
import * as authService from "../services/auth.service";

function httpStatusToGrpcCode(status: number): grpc.status {
  switch (status) {
    case 400:
      return grpc.status.INVALID_ARGUMENT;
    case 401:
      return grpc.status.UNAUTHENTICATED;
    case 403:
      return grpc.status.PERMISSION_DENIED;
    case 404:
      return grpc.status.NOT_FOUND;
    case 409:
      return grpc.status.ALREADY_EXISTS;
    default:
      return grpc.status.INTERNAL;
  }
}

function toGrpcError(err: unknown): grpc.ServiceError {
  const grpcErr = new Error(
    isHttpError(err) ? err.message : "Internal server error",
  ) as grpc.ServiceError;
  grpcErr.code = isHttpError(err) ? httpStatusToGrpcCode(err.status) : grpc.status.INTERNAL;
  return grpcErr;
}

function unary<Req, Res>(fn: (req: Req) => Promise<Res>): grpc.handleUnaryCall<Req, Res> {
  return (call, callback) => {
    fn(call.request)
      .then((result) => callback(null, result))
      .catch((err) => callback(toGrpcError(err)));
  };
}

export const authHandlers: AuthServiceServer = {
  register: unary(async (req) => {
    if (!req.email || !req.password) throw createHttpError(400, "email and password required");
    const result = await authService.register(req.email, req.username, req.password);
    return {
      accessToken: result.accessToken,
      refreshToken: result.refreshToken,
      user: {
        id: result.user.id,
        email: result.user.email,
        username: result.user.username ?? undefined,
      },
    };
  }),

  login: unary(async (req) => {
    if (!req.email || !req.password) throw createHttpError(400, "email and password required");
    const result = await authService.login(req.email, req.password);
    return {
      accessToken: result.accessToken,
      refreshToken: result.refreshToken,
      user: {
        id: result.user.id,
        email: result.user.email,
        username: result.user.username ?? undefined,
      },
    };
  }),

  refresh: unary(async (req) => {
    if (!req.refreshToken) throw createHttpError(400, "refreshToken required");
    return authService.refresh(req.refreshToken);
  }),

  logout: unary(async (req) => {
    if (!req.jti || !req.userId) throw createHttpError(400, "jti and userId required");
    await authService.logout(req.jti, req.exp, req.refreshToken, req.userId);
    return {};
  }),

  me: unary(async (req) => {
    if (!req.userId) throw createHttpError(400, "userId required");
    const user = await authService.me(req.userId);
    return {
      id: user.id,
      email: user.email,
      username: user.username ?? undefined,
      deleted: user.deleted,
      createdAt: user.created_at.toISOString(),
    };
  }),
};
