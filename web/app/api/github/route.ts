import { auth } from "@/lib/auth";
import { headers } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

const GITHUB_API = "https://api.github.com";

async function getAccessToken() {
  const session = await auth.api.getSession({
    headers: await headers(),
  });

  if (!session) {
    return null;
  }

  // Get the GitHub access token from the linked account
  const token = await auth.api.getAccessToken({
    body: {
      providerId: "github",
    },
    headers: await headers(),
  });

  return token?.accessToken || null;
}

async function githubFetch(path: string, token: string, accept = "application/vnd.github+json") {
  const res = await fetch(`${GITHUB_API}${path}`, {
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: accept,
      "X-GitHub-Api-Version": "2022-11-28",
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }));
    throw new Error(body.message || `GitHub API error: ${res.status}`);
  }

  return res;
}

// GET /api/github?action=repos|prs|diff&owner=...&repo=...&pr=...
export async function GET(req: NextRequest) {
  const token = await getAccessToken();
  if (!token) {
    return NextResponse.json({ error: "Not authenticated" }, { status: 401 });
  }

  const action = req.nextUrl.searchParams.get("action");

  try {
    switch (action) {
      case "repos": {
        const res = await githubFetch(
          "/user/repos?sort=pushed&per_page=30&type=all",
          token
        );
        const data = await res.json();
        return NextResponse.json(data);
      }

      case "prs": {
        const owner = req.nextUrl.searchParams.get("owner");
        const repo = req.nextUrl.searchParams.get("repo");
        if (!owner || !repo) {
          return NextResponse.json({ error: "Missing owner or repo" }, { status: 400 });
        }
        const res = await githubFetch(
          `/repos/${owner}/${repo}/pulls?state=open&sort=updated&per_page=30`,
          token
        );
        const data = await res.json();
        return NextResponse.json(data);
      }

      case "diff": {
        const owner = req.nextUrl.searchParams.get("owner");
        const repo = req.nextUrl.searchParams.get("repo");
        const pr = req.nextUrl.searchParams.get("pr");
        if (!owner || !repo || !pr) {
          return NextResponse.json({ error: "Missing owner, repo, or pr" }, { status: 400 });
        }
        const res = await githubFetch(
          `/repos/${owner}/${repo}/pulls/${pr}`,
          token,
          "application/vnd.github.diff"
        );
        const diffText = await res.text();
        return NextResponse.json({ diff: diffText });
      }

      default:
        return NextResponse.json({ error: "Unknown action" }, { status: 400 });
    }
  } catch (err) {
    return NextResponse.json(
      { error: (err as Error).message },
      { status: 500 }
    );
  }
}
