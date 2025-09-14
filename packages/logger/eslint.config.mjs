// packages/eslint-config/node.js
import config from "../eslint-config/node.js";

/**
 * Node.js-specific ESLint config extending the base monorepo rules.
 *
 * @type {import("eslint").Linter.Config[]}
 */
export default [...config];
