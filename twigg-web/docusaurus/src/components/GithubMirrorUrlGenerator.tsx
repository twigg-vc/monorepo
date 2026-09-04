import React, { useState } from "react";
import "../css/custom.css";

export default function GithubMirrorUrlGenerator() {
    const [token, setToken] = useState("");
    const [repoUrl, setRepoUrl] = useState("");

    function buildMirrorUrl(repoUrl: string, token: string) {
        if (!repoUrl) {
            return "https://<TOKEN>@<HOST>/<ORG>/<REPO>.git";
        }
        // Remove protocol (http:// or https://) and removes trailing /
        const stripped = repoUrl
            .replace(/^https?:\/\//, "")
            .replace(/\/+$/, "");
        // Ensure it ends with .git
        const withGit = stripped.endsWith(".git")
            ? stripped
            : `${stripped}.git`;
        const finalToken = token || "<TOKEN>";
        return `https://${finalToken}@${withGit}`;
    }

    const url = buildMirrorUrl(repoUrl.trim(), token.trim());

    return (
        <div className="git-mirror-url-generator-box">
            <label>
                <strong>Repository URL</strong>
                <input
                    className="git-mirror-url-generator-input"
                    type="text"
                    placeholder="https://github.com/org/repo or https://github.company.com/org/repo"
                    value={repoUrl}
                    onChange={(e) => setRepoUrl(e.target.value)}
                />
            </label>

            <label>
                <strong>Github Token</strong>
                <input
                    className="git-mirror-url-generator-input"
                    type="text"
                    placeholder="ghp_xxxxxxxx"
                    value={token}
                    onChange={(e) => setToken(e.target.value)}
                />
            </label>

            <pre className="git-mirror-url-generator-url-display">
                <code>{url}</code>
            </pre>

            <button
                className="git-mirror-url-generator-copy-btn"
                onClick={() => navigator.clipboard.writeText(url)}
            >
                Copy URL
            </button>
        </div>
    );
}