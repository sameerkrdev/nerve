export type TradeSide = "buy" | "sell";
export interface TradeData {
  id: string;
  engineTimestamp: string;
  clientTimestamp: string;
  symbol: string;
  price: number;
  volume: number;
  side: TradeSide;
  userId: string;
  createdAt: string;
}
