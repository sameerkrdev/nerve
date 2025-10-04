/* eslint-disable no-useless-catch */

import type { TradeRepository } from "@repo/database";
import type { Trade } from "@repo/types";

export default class TradeService {
  constructor(private tradeRepo: TradeRepository) {}

  async createTrade({
    engineTimestamp,
    clientTimestamp,
    symbol,
    price,
    volume,
    side,
    userId,
  }: Trade) {
    try {
      await this.tradeRepo.create({
        engineTimestamp,
        clientTimestamp,
        symbol,
        price,
        volume,
        side,
        userId,
      });
    } catch (error) {
      throw error;
    }
  }
}
