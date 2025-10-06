import type UserService from "@/services/user.service";
import type { Logger } from "@repo/logger";
import type { Response, NextFunction } from "express";
import type {
  CreateUserRequest,
  ListUsersRequest,
  UpdateUserRequest,
  UserIdRequest,
} from "@/types";

export default class UserController {
  constructor(
    private userService: UserService,
    private logger: Logger,
  ) {}

  // Create user
  async createUser(req: CreateUserRequest, res: Response, next: NextFunction) {
    try {
      const { name, email, password } = req.body;
      await this.userService.createUser({ name, email, password });
      this.logger.info("New user is created", { name, email });
      res.status(201).json({ message: "New user is created" });
    } catch (error) {
      next(error);
    }
  }

  // Get user by ID
  async getUser(req: UserIdRequest, res: Response, next: NextFunction) {
    try {
      const user = await this.userService.getUserById(req.params.id);

      if (!user) {
        return res.status(404).json({ message: "User not found" });
      }

      return res.status(200).json(user);
    } catch (error) {
      next(error);
      return;
    }
  }

  // Update user
  async updateUser(req: UpdateUserRequest, res: Response, next: NextFunction) {
    try {
      const { id } = req.params;
      const updatedUser = await this.userService.updateUser(id, req.body);
      res.json(updatedUser);
    } catch (error) {
      next(error);
    }
  }

  // Delete user
  async deleteUser(req: { params: { id: string } }, res: Response, next: NextFunction) {
    try {
      await this.userService.deleteUser(req.params.id);
      res.json({ message: "User deleted successfully" });
    } catch (error) {
      next(error);
    }
  }

  // List users
  async listUsers(req: ListUsersRequest, res: Response, next: NextFunction) {
    try {
      const skip = Number(req.query.skip || 0);
      const take = Number(req.query.take || 10);
      const users = await this.userService.listUsers({ skip, take });
      res.json(users);
    } catch (error) {
      next(error);
    }
  }
}
