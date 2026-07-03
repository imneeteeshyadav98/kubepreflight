// Shared clipboard helper for the Console's several "Copy ..." buttons
// (remediation, finding JSON). Returns a status string the caller renders
// directly as button text, rather than throwing, since a failed copy is a
// normal, expected outcome in browsers without clipboard permission.
export async function copyToClipboard(text: string): Promise<"Copied" | "Copy unavailable"> {
  try {
    await navigator.clipboard.writeText(text);
    return "Copied";
  } catch {
    return "Copy unavailable";
  }
}
