import express, { type Response, type NextFunction, type Router } from "express";
import * as grpc from "@grpc/grpc-js";
import { OrderServiceClient } from "@repo/proto-defs/ts/order_service";

import { OrderController } from "@/controllers/order.controller";
import { logger } from "@repo/logger";
import type { CreateOrderRequest } from "@/types";
import { PlaceOrderValidator } from "@repo/validator";
import zodValidatorMiddleware from "@/middlewares/zod.validator.middleware";
import env from "@/config/dotenv";

const router: Router = express.Router();

const ORDER_SERVICE_GRPC_URL = env.ORDER_SERVICE_GRPC_URL;
const credentials =
  process.env.NODE_ENV === "production"
    ? grpc.credentials.createSsl()
    : grpc.credentials.createInsecure();

const orderClient = new OrderServiceClient(ORDER_SERVICE_GRPC_URL, credentials, {
  "grpc.keepalive_time_ms": 30000,
  "grpc.keepalive_timeout_ms": 10000,
});

logger.info(`Connected to Order gRPC service at ${ORDER_SERVICE_GRPC_URL}`);

const orderController = new OrderController(logger, orderClient);

router.post(
  "/",
  zodValidatorMiddleware(PlaceOrderValidator),
  (req: CreateOrderRequest, res: Response, next: NextFunction) =>
    orderController.createOrder(req, res, next),
);

export default router;
