import type { Request } from "express";
import type { Trade, User } from "@repo/types";

// For creating a user
export interface CreateTradeRequest extends Request {
  body: Trade;
}

// For creating a user
export interface CreateUserRequest extends Request {
  body: User;
}

// For updating a user
export interface UpdateUserRequest extends Request {
  params: { id: string };
  body: Partial<User>; // allow partial update
}

// For fetching/deleting a user by ID
export interface UserIdRequest extends Request {
  params: { id: string };
}

export interface UserIdsRequest extends Request {
  body: { ids: string[] };
}

// For listing users with optional pagination
export interface ListUsersRequest extends Request {
  query: {
    skip?: string;
    take?: string;
  };
}

// Express request body
type Order = {
  symbol: string;
  price: number;
  quantity: number;
  side: "BUY" | "SELL";
  type: "MARKET" | "LIMIT";
  userId: string;
};

export interface CreateOrderRequest extends Request {
  body: Order;
}
