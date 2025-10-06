import type { Trade, User } from "@repo/types";
import type { Request } from "express";

export interface CreateTradeRequest extends Request {
  body: Trade;
}

export interface CreateUserRequest extends Request {
  body: User;
}
