const path = require("path");

const toPosix = (p) => p.replace(/\\/g, "/");

const quote = (p) => `"${toPosix(p)}"`;

module.exports = {
  // Handle apps separately with proper directory context
  "apps/**/*.{js,jsx,ts,tsx}": (filenames) => {
    const filesByApp = filenames.reduce((acc, filename) => {
      const match = filename.match(/(?:^|[\\/])apps[\\/]([^\\/]+)[\\/](.+)/);
      if (!match) return acc;

      const appName = match[1];
      if (!acc[appName]) acc[appName] = [];
      acc[appName].push(filename);
      return acc;
    }, {});

    return Object.entries(filesByApp).flatMap(([appName, files]) => {
      const appDir = path.join("apps", appName);
      const relativeFiles = files.map((file) => path.relative(appDir, file));
      const fileArgs = relativeFiles.map(quote).join(" ");

      console.log(`🔍 Linting ${files.length} staged files in ${toPosix(appDir)}:`, relativeFiles);

      return [
        `pnpm --dir ${quote(appDir)} exec eslint --fix ${fileArgs}`,
        `pnpm --dir ${quote(appDir)} exec prettier --write ${fileArgs}`,
      ];
    });
  },

  // Handle packages separately
  "packages/**/*.{js,jsx,ts,tsx}": (filenames) => {
    const filesByPackage = filenames.reduce((acc, filename) => {
      const match = filename.match(/(?:^|[\\/])packages[\\/]([^\\/]+)[\\/](.+)/);
      if (!match) return acc;

      if (
        match[1] == "eslint-config" ||
        match[1] == "typescript-config" ||
        match[1] === "types" ||
        match[1] === "proto-defs"
      )
        return acc;

      const pkgName = match[1];
      if (!acc[pkgName]) acc[pkgName] = [];
      acc[pkgName].push(filename);
      return acc;
    }, {});

    return Object.entries(filesByPackage).flatMap(([pkgName, files]) => {
      const pkgDir = path.join("packages", pkgName);
      const relativeFiles = files.map((file) => path.relative(pkgDir, file));
      const fileArgs = relativeFiles.map(quote).join(" ");

      console.log(`📦 Linting ${files.length} staged files in ${toPosix(pkgDir)}:`, relativeFiles);

      return [
        `pnpm --dir ${quote(pkgDir)} exec eslint --fix ${fileArgs}`,
        `pnpm --dir ${quote(pkgDir)} exec prettier --write ${fileArgs}`,
      ];
    });
  },

  // Root level files
  "*.{json,md,yml,yaml}": ["prettier --write"],
};
