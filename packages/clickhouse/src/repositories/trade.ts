import crypto from "node:crypto";
import { clickHouseManager } from "../index";
import type { Trade } from "@repo/types";

export class TradeRepository {
  private get client() {
    return clickHouseManager.getClickhouseClient();
  }

  async create(tradeData: Trade): Promise<string> {
    const tradeId = crypto.randomUUID();

    await this.client.insert({
      table: "nerve.trade_data",
      format: "JSONEachRow",
      values: [
        {
          id: tradeId,
          client_timestamp: tradeData.clientTimestamp,
          engine_timestamp: tradeData.engineTimestamp,
          symbol: tradeData.symbol,
          price: tradeData.price,
          volume: tradeData.volume,
          side: tradeData.side,
          user_id: tradeData.userId,
        },
      ],
    });

    return tradeId;
  }

  async findById(id: string): Promise<Trade | null> {
    const resultSet = await this.client.query({
      query: "SELECT * FROM nerve.trade_data WHERE id = {id:UUID} LIMIT 1",
      query_params: { id },
      format: "JSONEachRow",
    });

    for await (const rows of resultSet.stream()) {
      for (const row of rows) {
        return JSON.parse(row.text) as Trade;
      }
    }

    return null;
  }
}
