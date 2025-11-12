/* eslint-disable @typescript-eslint/no-explicit-any */
import type { NextFunction, Request, Response } from "express";
import { z } from "@repo/validator";
import createHttpError from "http-errors";

const zodValidatorMiddleware = <T extends z.ZodTypeAny>(schema: T) => {
  return (req: Request, _res: Response, next: NextFunction) => {
    const result = schema.safeParse({
      body: req.body,
      query: req.query,
      params: req.params,
    });

    if (!result.success) {
      const error = createHttpError(400, z.treeifyError(result.error));
      return next(error);
    }

    const { body, query, params } = result.data as {
      body?: unknown;
      query?: unknown;
      params?: unknown;
    };

    if (body !== undefined) req.body = body;
    if (query !== undefined) req.query = query as any;
    if (params !== undefined) req.params = params as any;

    next();
  };
};

export default zodValidatorMiddleware;
