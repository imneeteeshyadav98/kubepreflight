// Shared clipboard helper for the Console's several "Copy ..." buttons
// (remediation, finding JSON). Returns a status string the caller renders
// directly as button text, rather than throwing, since a failed copy is a
// normal, expected outcome in browsers without clipboard permission.
export async function copyToClipboard(text: string): Promise<"Copied" | "Copy unavailable"> {
  try {
	if (!navigator.clipboard?.writeText) throw new Error("Clipboard API unavailable");
	await navigator.clipboard.writeText(text);
	return "Copied";
  } catch {
	const area = document.createElement("textarea");
	area.value = text;
	area.style.position = "fixed";
	area.style.opacity = "0";
	document.body.appendChild(area);
	area.select();
	let copied = false;
	try { copied = document.execCommand("copy"); } catch { copied = false; }
	area.remove();
	return copied ? "Copied" : "Copy unavailable";
  }
}
