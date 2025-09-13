// packages/eslint-config/node.js
import { config as baseConfig } from "./base.js";
import tseslint from "typescript-eslint";
import globals from "globals";

/**
 * Node.js-specific ESLint config extending the base monorepo rules.
 *
 * @type {import("eslint").Linter.Config[]}
 */
export default [
  ...baseConfig,
  {
    files: ["**/*.ts"],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        project: true, // <-- auto-detects the nearest tsconfig.json
      },
      globals: {
        ...globals.node,
      },
    },
    rules: {
      "no-console": "off",
      "@typescript-eslint/no-explicit-any": "warn",
      "@typescript-eslint/consistent-type-imports": "error",
    },
  },
];
