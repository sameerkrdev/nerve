import type UserService from "@/services/user.service";
import type { Logger } from "@repo/logger";
import type { Response, NextFunction } from "express";
import type {
  CreateUserRequest,
  ListUsersRequest,
  UpdateUserRequest,
  UserIdRequest,
  UserIdsRequest,
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
      if (!user) return res.status(404).json({ message: "User not found" });
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

  // Soft delete user
  async softDeleteUser(req: UserIdRequest, res: Response, next: NextFunction) {
    try {
      await this.userService.softDeleteUser(req.params.id);
      res.json({ message: "User soft deleted successfully" });
    } catch (error) {
      next(error);
    }
  }

  // Hard delete many users
  async softDeleteManyUsers(req: UserIdsRequest, res: Response, next: NextFunction) {
    try {
      const { ids } = req.body;
      await this.userService.softDeleteMany(ids);
      res.json({ message: "Users soft deleted" });
    } catch (error) {
      next(error);
    }
  }

  // Hard delete (single user)
  async hardDeleteUser(req: UserIdRequest, res: Response, next: NextFunction) {
    try {
      await this.userService.hardDeleteUser(req.params.id);
      res.json({ message: "User permanently deleted" });
    } catch (error) {
      next(error);
    }
  }

  // Hard delete many users
  async hardDeleteManyUsers(req: UserIdsRequest, res: Response, next: NextFunction) {
    try {
      const { ids } = req.body; // expecting an array of IDs
      await this.userService.hardDeleteMany(ids);
      res.json({ message: "Users permanently deleted" });
    } catch (error) {
      next(error);
    }
  }

  // List users
  async listUsers(req: ListUsersRequest, res: Response, next: NextFunction) {
    try {
      const skip = Math.max(0, parseInt(req.query.skip as string, 10) || 0);
      const take = Math.max(1, Math.min(100, parseInt(req.query.take as string, 10) || 10));

      const users = await this.userService.listUsers({ skip, take });
      res.json(users);
    } catch (error) {
      next(error);
    }
  }
}
