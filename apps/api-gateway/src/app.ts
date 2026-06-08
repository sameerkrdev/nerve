import express, { type Express, type NextFunction, type Request, type Response } from "express";
import type { HttpError } from "http-errors";
import { logger } from "@repo/logger";
import userRouter from "@/routers/user.route";
import orderRouter from "@/routers/order.route";
import authRouter from "@/routers/auth.route";
import { authMiddleware } from "@/middlewares/auth.middleware";

const app: Express = express();

app.use(express.json());

app.use("/auth", authRouter);
app.use("/api/v1/users", userRouter);
app.use("/api/v1/orders", authMiddleware, orderRouter);

app.get("/", (_, res) => {
  res.json({ message: "Hello World from Nerve trade platform's backend" });
});

// eslint-disable-next-line @typescript-eslint/no-unused-vars
app.use((err: HttpError, req: Request, res: Response, _next: NextFunction) => {
  if (err instanceof Error) {
    const statusCode = err.status || err.statusCode || 500;

    logger.error({
      message: err.message,
      name: err.name,
      stack: err.stack,
      method: req.method,
      path: req.originalUrl,
      params: req.params,
      query: req.query,
      body: { ...req.body, password: null },
    });

    res.status(statusCode).json({
      error: [
        {
          type: err.name,
          msg: err.message,
          method: req.method,
          path: req.originalUrl,
        },
      ],
    });
  }
});

export default app;
