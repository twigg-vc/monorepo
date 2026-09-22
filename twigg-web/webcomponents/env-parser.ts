export interface EnvEntry {
    Name: string
    Value: string
}

// Parses text in dotenv format ("KEY=value" per line) into entries.
// Supports "export KEY=value", "# comments", blank lines, single and double
// quoted values, and a trailing " # comment" on unquoted values.
// Lines without "=" are ignored.
export function ParseEnv(text: string): EnvEntry[] {
    var entries: EnvEntry[] = []
    const lines = text.split(/\r?\n/)
    for (const rawLine of lines) {
        var line = rawLine.trim()
        if (line === "" || line.startsWith("#")) {
            continue
        }
        if (line.startsWith("export ")) {
            line = line.substring("export ".length).trim()
        }
        const eq = line.indexOf("=")
        if (eq < 0) {
            continue
        }
        const name = line.substring(0, eq).trim()
        if (name === "") {
            continue
        }
        const value = parseValue(line.substring(eq + 1).trim())
        entries.push({ Name: name, Value: value })
    }
    return entries
}

function parseValue(raw: string): string {
    if (raw.length >= 2) {
        const first = raw[0]
        const last = raw[raw.length - 1]
        if ((first === '"' || first === "'") && first === last) {
            return raw.substring(1, raw.length - 1)
        }
    }
    const hash = raw.indexOf(" #")
    if (hash >= 0) {
        return raw.substring(0, hash).trim()
    } else {
        return raw
    }
}

// Reports whether pasted text looks like a dotenv block rather than a single
// secret name: it has more than one line or contains a "=".
export function LooksLikeEnv(text: string): boolean {
    return text.includes("=") || text.trim().includes("\n")
}
