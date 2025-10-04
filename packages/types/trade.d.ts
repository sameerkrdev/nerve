export type TradeSide = "buy" | "sell";

export interface Trade {
  clientTimestamp: string;
  engineTimestamp: string;
  symbol: string;
  price: number;
  volume: number;
  side: TradeSide;
  userId: string;
}

export interface TradeData extends Trade {
  id: string;
  createdAt: string;
}
