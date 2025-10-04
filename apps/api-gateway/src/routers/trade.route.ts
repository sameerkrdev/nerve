import type { NextFunction, Request, Response, Router } from "express";
import express from "express";
import { TradeRepository } from "@repo/clickhouse";
import { logger } from "@repo/logger";

import TradeController from "@/controllers/trade.controller";
import TradeService from "@/services/trade.service";
import type { CreateTradeRequest } from "@/types";

const router: Router = express.Router();

const tradeRepo = new TradeRepository();
const tradeService = new TradeService(tradeRepo);
const tradeController = new TradeController(tradeService, logger);

router.route("/").post((req: Request, res: Response, next: NextFunction) => {
  tradeController.createTrade(req as unknown as CreateTradeRequest, res, next);
});

export default router;
