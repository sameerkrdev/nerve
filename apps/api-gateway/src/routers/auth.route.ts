import express, { type Router, type Request, type Response, type NextFunction } from "express";
import rateLimit from "express-rate-limit";
import * as grpc from "@grpc/grpc-js";
import createHttpError from "http-errors";
import { authClient } from "@/grpc/auth.client";
import { authMiddleware } from "@/middlewares/auth.middleware";

const router: Router = express.Router();

const strictLimiter = rateLimit({
  windowMs: 15 * 60 * 1000,
  max: 10,
  standardHeaders: true,
  legacyHeaders: false,
  message: { error: [{ msg: "Too many requests, try again later" }] },
});

function grpcCodeToHttp(code: grpc.status): number {
  switch (code) {
    case grpc.status.INVALID_ARGUMENT:
      return 400;
    case grpc.status.UNAUTHENTICATED:
      return 401;
    case grpc.status.PERMISSION_DENIED:
      return 403;
    case grpc.status.NOT_FOUND:
      return 404;
    case grpc.status.ALREADY_EXISTS:
      return 409;
    default:
      return 500;
  }
}

function handleGrpcErr(err: grpc.ServiceError, next: NextFunction) {
  next(createHttpError(grpcCodeToHttp(err.code ?? grpc.status.INTERNAL), err.message));
}

router.post("/register", strictLimiter, (req: Request, res: Response, next: NextFunction) => {
  authClient.register(
    { email: req.body.email, username: req.body.username, password: req.body.password },
    (err, result) => {
      if (err) return handleGrpcErr(err, next);
      res.status(201).json(result);
    },
  );
});

router.post("/login", strictLimiter, (req: Request, res: Response, next: NextFunction) => {
  authClient.login({ email: req.body.email, password: req.body.password }, (err, result) => {
    if (err) return handleGrpcErr(err, next);
    res.json(result);
  });
});

router.post("/refresh", (req: Request, res: Response, next: NextFunction) => {
  authClient.refresh({ refreshToken: req.body.refreshToken }, (err, result) => {
    if (err) return handleGrpcErr(err, next);
    res.json(result);
  });
});

router.post("/logout", authMiddleware, (req: Request, res: Response, next: NextFunction) => {
  authClient.logout(
    {
      jti: req.user!.jti,
      exp: req.user!.exp,
      refreshToken: req.body.refreshToken,
      userId: req.user!.id,
    },
    (err) => {
      if (err) return handleGrpcErr(err, next);
      res.status(204).end();
    },
  );
});

router.get("/me", authMiddleware, (req: Request, res: Response, next: NextFunction) => {
  authClient.me({ userId: req.user!.id }, (err, result) => {
    if (err) return handleGrpcErr(err, next);
    res.json(result);
  });
});

export default router;
