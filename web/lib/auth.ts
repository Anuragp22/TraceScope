import { betterAuth } from "better-auth";

// NOTE ON PERSISTENCE — the dashboard is a demo surface (see the project README).
// `secret` is read from the environment so sessions survive a server restart
// instead of being signed with a random per-process key. There is intentionally
// no `database` adapter here: without one, better-auth keeps sessions and linked
// accounts in memory, so the GitHub token obtained during sign-in does not
// persist across restarts. That is acceptable for a local demo but NOT for
// production. A production deployment that reads pull requests and posts review
// comments should be a **GitHub App** using installation tokens (not an OAuth
// app — an OAuth app cannot create check runs), backed by a real database
// adapter for token/account storage.
export const auth = betterAuth({
  secret: process.env.BETTER_AUTH_SECRET,
  socialProviders: {
    github: {
      clientId: process.env.GITHUB_CLIENT_ID!,
      clientSecret: process.env.GITHUB_CLIENT_SECRET!,
      scope: ["repo", "read:user", "user:email"],
    },
  },
  account: {
    accountLinking: {
      enabled: true,
    },
  },
});
