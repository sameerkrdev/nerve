import type { TradeRepository } from "@repo/clickhouse";
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
    await this.tradeRepo.create({
      engineTimestamp,
      clientTimestamp,
      symbol,
      price,
      volume,
      side,
      userId,
    });
  }
}
