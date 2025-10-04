import type { Trade } from "@repo/types";

export interface CreateTradeRequest extends Request {
  body: Trade;
}
