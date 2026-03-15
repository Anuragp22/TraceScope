const GITHUB_API = "https://api.github.com";

export interface GitHubRepo {
  id: number;
  full_name: string;
  name: string;
  owner: { login: string };
  private: boolean;
  default_branch: string;
}

export interface GitHubPR {
  number: number;
  title: string;
  state: string;
  head: { ref: string; sha: string };
  base: { ref: string; sha: string };
  user: { login: string; avatar_url: string };
  created_at: string;
  updated_at: string;
}

export interface GitHubBranch {
  name: string;
  commit: { sha: string };
}

async function githubFetch<T>(path: string, token: string): Promise<T> {
  const res = await fetch(`${GITHUB_API}${path}`, {
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/vnd.github+json",
      "X-GitHub-Api-Version": "2022-11-28",
    },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }));
    throw new Error(body.message || `GitHub API error: ${res.status}`);
  }
  return res.json();
}

async function githubFetchText(path: string, token: string, accept: string): Promise<string> {
  const res = await fetch(`${GITHUB_API}${path}`, {
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: accept,
      "X-GitHub-Api-Version": "2022-11-28",
    },
  });
  if (!res.ok) {
    throw new Error(`GitHub API error: ${res.status}`);
  }
  return res.text();
}

export const github = {
  // Get user's repos (sorted by recent push)
  getRepos: (token: string, page = 1) =>
    githubFetch<GitHubRepo[]>(
      `/user/repos?sort=pushed&per_page=30&page=${page}&type=all`,
      token
    ),

  // Get open PRs for a repo
  getPRs: (token: string, owner: string, repo: string) =>
    githubFetch<GitHubPR[]>(
      `/repos/${owner}/${repo}/pulls?state=open&sort=updated&per_page=30`,
      token
    ),

  // Get branches for a repo
  getBranches: (token: string, owner: string, repo: string) =>
    githubFetch<GitHubBranch[]>(
      `/repos/${owner}/${repo}/branches?per_page=100`,
      token
    ),

  // Get PR diff as unified diff text
  getPRDiff: (token: string, owner: string, repo: string, prNumber: number) =>
    githubFetchText(
      `/repos/${owner}/${repo}/pulls/${prNumber}`,
      token,
      "application/vnd.github.diff"
    ),

  // Get diff between two branches
  getBranchDiff: (token: string, owner: string, repo: string, base: string, head: string) =>
    githubFetchText(
      `/repos/${owner}/${repo}/compare/${base}...${head}`,
      token,
      "application/vnd.github.diff"
    ),
};
