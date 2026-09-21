import frontendConfig from "../../frontend/eslint.config.js"

export default [
  ...frontendConfig,
  {
    files: [
      "src/auth-context.tsx",
      "src/dashboard-context.tsx",
      "src/host-context.tsx",
    ],
    rules: {
      // These modules intentionally export a Provider and its matching hook.
      "react-refresh/only-export-components": "off",
    },
  },
  {
    files: ["src/auth-context.tsx", "src/pages/company-page.tsx"],
    rules: {
      // Existing initialization effects are kept unchanged during this refactor.
      "react-hooks/set-state-in-effect": "off",
    },
  },
  {
    files: ["src/**/*.{ts,tsx}"],
    rules: {
      "no-restricted-imports": ["error", {
        patterns: [{
          group: ["@/**", "@frontend/**", "**/frontend/src/**"],
          message: "Dashboard must not import from the self-hosted app.",
        }],
      }],
    },
  },
]
