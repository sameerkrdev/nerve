import type UserService from "@/services/user.service";
import type { CreateUserRequest } from "@/types";
import type { Logger } from "@repo/logger";
import type { Response, NextFunction } from "express";

export default class UserController {
  constructor(
    private userService: UserService,
    private logger: Logger,
  ) {}

  async createUser(req: CreateUserRequest, res: Response, next: NextFunction) {
    try {
      const { name, email, password } = req.body;

      await this.userService.createUser({
        name,
        email,
        password,
      });

      this.logger.info("New user is created", {
        name,
        email,
        password: "****",
      });
      res.status(201).json({ message: "New user is created" });
    } catch (error) {
      return next(error);
    }
  }
}
