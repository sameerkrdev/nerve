import type { Request } from "express";
import type { Trade, User } from "@repo/types";
import type { PlaceOrder, CancelOrder, ModifyOrder } from "@repo/validator";

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

// type OrderSideKeys = keyof typeof Side;
// type OrderTypeKeys = keyof typeof Type;

// Express request body
// type Order = {
//   symbol: string;
//   price: number;
//   quantity: number;
//   side: OrderSideKeys;
//   type: OrderTypeKeys;
// };

export interface CreateOrderRequest extends Request {
  body: PlaceOrder["body"];
}

export interface CancelOrderRequest extends Request {
  params: CancelOrder["params"];
  body: CancelOrder["body"];
}

export interface ModifyOrderRequest extends Request {
  params: ModifyOrder["params"];
  body: ModifyOrder["body"];
}
