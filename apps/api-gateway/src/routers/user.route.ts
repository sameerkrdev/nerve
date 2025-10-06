import type { NextFunction, Request, Response, Router } from "express";
import express from "express";
import { UserRepository } from "@repo/prisma";
import { logger } from "@repo/logger";

import UserController from "@/controllers/user.controller";
import UserService from "@/services/user.service";
import type { CreateUserRequest } from "@/types";

const router: Router = express.Router();

const userRepo = new UserRepository();
const userService = new UserService(userRepo);
const userController = new UserController(userService, logger);

router.route("/").post((req: Request, res: Response, next: NextFunction) => {
  userController.createUser(req as unknown as CreateUserRequest, res, next);
});

export default router;
