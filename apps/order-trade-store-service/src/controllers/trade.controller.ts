import type { Logger } from "@repo/logger";
import { type TradeRepository } from "@repo/prisma";

export class TradeServerController {
  constructor(
    private readonly logger: Logger,
    private tradeRepo: TradeRepository,
  ) {}

  async createTrade(data: {
    id: string;
    symbol: string;
    tradeSequence: number;
    price: number;
    quantity: number;
    buyerId: string;
    sellerId: string;
    buyOrderId: string;
    sellOrderId: string;
    timestamp: Date;
    isBuyerMaker: boolean;
  }) {
    try {
      if (!this.tradeRepo) {
        this.logger.warn("TradeRepository not provided, skipping order persistence");
        return;
      }

      const newTrade = await this.tradeRepo.create({
        engine_id: data.id,
        symbol: data.symbol,
        price: data.price,
        quantity: data.quantity,
        trade_sequence: data.tradeSequence,
        is_buyer_maker: data.isBuyerMaker,

        sell_order: { connect: { id: data.sellOrderId } },
        buy_order: { connect: { id: data.buyOrderId } },

        buyer: { connect: { id: data.buyerId } },
        seller: { connect: { id: data.sellerId } },
      });

      this.logger.info("trade persisted successfully", { orderId: newTrade.id });
    } catch (error) {
      this.logger.error("Failed to persist trade", {
        message: error instanceof Error ? error.message : String(error),
      });

      throw error;
    }
  }
}
