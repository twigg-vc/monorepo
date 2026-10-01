// Cláudio wrote this regex. It looks dense, but it's well tested in
// md-anchors.test.ts. see that file if you need to understand a case.
const ANCHOR_RE = /(```[\s\S]*?```)|(`[^`]*`)|(\[[^\]]*\]\([^)]*\))|(?<![\w/])c\/(\d+)(?:v(\d+))?\b|(?<![\w/])b\/(\d+)\b/gi;

// Takes raw markdown plus the repo's owner and name, and returns that
// markdown with commit refs (c/7) and bug refs (b/17) rewritten into links,
// e.g. "c/7" -> "[c/7](/owner/repo/c/7)".
export function LinkifyCommitsAndBugs(markdown: string, owner: string, repo: string): string {
    return markdown.replace(ANCHOR_RE,
        (match, fencedBlock, inlineCode, existingLink, commitId, _commitVersion, bugId) => {
            if (fencedBlock !== undefined || inlineCode !== undefined || existingLink !== undefined) {
                return match;
            }
            if (commitId !== undefined) {
                return `[${match}](/${owner}/${repo}/c/${commitId})`;
            }
            return `[${match}](/${owner}/${repo}/b/${bugId})`;
        });
}
