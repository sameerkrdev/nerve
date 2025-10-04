import type { Trade } from "@repo/types";
import type { Request } from "express";

export interface CreateTradeRequest extends Request {
  body: Trade;
}
