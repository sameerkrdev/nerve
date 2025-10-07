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
} from "@/types";

const router: Router = express.Router();

const userRepo = new UserRepository();
const userService = new UserService(userRepo);
const userController = new UserController(userService, logger);

// CREATE USER
router.post("/", (req: CreateUserRequest, res: Response, next: NextFunction) =>
  userController.createUser(req, res, next),
);

// GET USER BY ID
router.get("/:id", (req: UserIdRequest, res: Response, next: NextFunction) =>
  userController.getUser(req, res, next),
);

// UPDATE USER
router.put("/:id", (req: UpdateUserRequest, res: Response, next: NextFunction) =>
  userController.updateUser(req, res, next),
);

// DELETE USER
router.delete("/:id", (req: UserIdRequest, res: Response, next: NextFunction) =>
  userController.deleteUser(req, res, next),
);

// LIST USERS
router.get("/", (req: ListUsersRequest, res: Response, next: NextFunction) =>
  userController.listUsers(req, res, next),
);

export default router;
