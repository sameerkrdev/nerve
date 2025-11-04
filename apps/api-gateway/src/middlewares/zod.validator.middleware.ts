/* eslint-disable @typescript-eslint/no-explicit-any */
import type { NextFunction, Request, Response } from "express";
import { z } from "@repo/validator";

const zodValidatorMiddleware = <T extends z.ZodTypeAny>(schema: T) => {
  return (req: Request, _res: Response, next: NextFunction) => {
    const result = schema.safeParse({
      body: req.body,
      query: req.query,
      params: req.params,
    });

    if (!result.success) {
      return next(new Error(JSON.stringify(z.treeifyError(result.error))));
    }

    const { body, query, params } = result.data as {
      body?: unknown;
      query?: unknown;
      params?: unknown;
    };

    if (body) req.body = body;
    if (query) req.query = query as any;
    if (params) req.params = params as any;

    next();
  };
};

export default zodValidatorMiddleware;
