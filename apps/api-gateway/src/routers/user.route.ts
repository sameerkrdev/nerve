import type { NextFunction, Response, Router } from "express";
import express from "express";
import { UserRepository } from "@repo/prisma";
import { logger } from "@repo/logger";

import UserController from "@/controllers/user.controller";
import UserService from "@/services/user.service";
import type {
  CreateUserRequest,
  UpdateUserRequest,
  UserIdRequest,
  ListUsersRequest,
  UserIdsRequest,
} from "@/types";

const router: Router = express.Router();

// Instantiate layers
const userRepo = new UserRepository();
const userService = new UserService(userRepo);
const userController = new UserController(userService, logger);

// CREATE USER
router.post("/", (req: CreateUserRequest, res: Response, next: NextFunction) =>
  userController.createUser(req, res, next),
);

// LIST USERS
router.get("/", (req: ListUsersRequest, res: Response, next: NextFunction) =>
  userController.listUsers(req, res, next),
);

// GET USER BY ID
router.get("/:id", (req: UserIdRequest, res: Response, next: NextFunction) =>
  userController.getUser(req, res, next),
);

// UPDATE USER
router.put("/:id", (req: UpdateUserRequest, res: Response, next: NextFunction) =>
  userController.updateUser(req, res, next),
);

// SOFT DELETE USER
router.delete("/soft/:id", (req: UserIdRequest, res: Response, next: NextFunction) =>
  userController.softDeleteUser(req, res, next),
);

// SOFT DELETE  MULTIPLE USERS
router.delete("/soft", (req: UserIdsRequest, res: Response, next: NextFunction) =>
  userController.softDeleteManyUsers(req, res, next),
);

// HARD DELETE USER (single)
router.delete("/hard/:id", (req: UserIdRequest, res: Response, next: NextFunction) =>
  userController.hardDeleteUser(req, res, next),
);

// HARD DELETE MULTIPLE USERS
router.delete("/hard", (req: UserIdsRequest, res: Response, next: NextFunction) =>
  userController.hardDeleteManyUsers(req, res, next),
);

export default router;
