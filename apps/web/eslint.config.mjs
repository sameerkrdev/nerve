// apps/web/eslint.config.js
import globals from "globals";
import reactRefresh from "eslint-plugin-react-refresh";
import { globalIgnores } from "eslint/config";
import { config as baseConfig } from "@repo/eslint-config/react-internal";

export default [
  ...baseConfig,
  globalIgnores(["dist"]),
  {
    files: ["**/*.{ts,tsx}"],
    ...reactRefresh.configs.vite,
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
  },
];
