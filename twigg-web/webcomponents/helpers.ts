function normalizeUtc(ts: string): Date {
    if (!ts.includes("T")) {
        ts = ts.replace(" ", "T") + "Z";
    }
    return new Date(ts);
}

export function FormatDateTime(ts: string): string {
    const date = normalizeUtc(ts);

    return new Intl.DateTimeFormat("en-US", {
        month: "short",
        day: "2-digit",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
        hour12: false,
    }).format(date)
}

export function FormatRelativeTime(createdOn: string): string {
    const created = new Date(createdOn);
    const now = new Date();

    let diffMs = now.getTime() - created.getTime();

    if (diffMs < 0) diffMs = 0;

    const diffSec = Math.floor(diffMs / 1000);
    const diffMin = Math.floor(diffSec / 60);
    const diffHour = Math.floor(diffMin / 60);
    const diffDay = Math.floor(diffHour / 24);
    const diffWeek = Math.floor(diffDay / 7);

    if (diffSec < 30) {
        return "just now";
    }
    if (diffMin < 1) {
        return `${diffSec}s ago`;
    }
    if (diffMin < 60) {
        return diffMin === 1 ? "1 min ago" : `${diffMin} mins ago`;
    }
    if (diffHour < 24) {
        return diffHour === 1 ? "1h ago" : `${diffHour}h ago`;
    }
    if (diffDay < 7) {
        return diffDay === 1 ? "1 day ago" : `${diffDay} days ago`;
    }
    if (diffWeek < 5) {
        return diffWeek === 1 ? "1 week ago" : `${diffWeek} weeks ago`;
    }

    // Fallback: older than ~1 month, use a short absolute date
    const monthNames = ["Jan", "Feb", "Mar", "Apr", "May", "Jun",
        "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

    const month = monthNames[created.getMonth()];
    const day = String(created.getDate()).padStart(2, "0");
    const year = created.getFullYear();

    return `${day} ${month} ${year}`;
}