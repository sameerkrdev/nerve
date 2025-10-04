import type TradeService from "@/services/trade.service";
import type { CreateTradeRequest } from "@/types";
import type { Logger } from "@repo/logger";
import type { Response, NextFunction } from "express";

export default class TradeController {
  constructor(
    private TradeService: TradeService,
    private logger: Logger,
  ) {}

  async createTrade(req: CreateTradeRequest, res: Response, next: NextFunction) {
    try {
      const { engineTimestamp, clientTimestamp, symbol, price, volume, side, userId } = req.body;

      await this.TradeService.createTrade({
        engineTimestamp,
        clientTimestamp,
        symbol,
        price,
        volume,
        side,
        userId,
      });

      this.logger.info("New trade is created", {
        engineTimestamp,
        clientTimestamp,
        symbol,
        price,
        volume,
        side,
        userId,
      });
      res.status(201).json({ message: "New trade is created" });
    } catch (error) {
      return next(error);
    }
  }
}
