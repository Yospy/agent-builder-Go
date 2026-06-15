// V1 tool catalog metadata, mirrored from docs/00-CONTEXT.md §6. The API only
// exposes tool NAMES, so descriptions and the consequential flag (which tools
// pause for approval) live here. Unknown names fall back gracefully.

export interface ToolInfo {
  description: string;
  consequential: boolean;
}

const TOOL_CATALOG: Record<string, ToolInfo> = {
  // Platform tools
  list_tools: { description: "Browse the tool catalog", consequential: false },
  list_agents: { description: "List existing agents", consequential: false },
  ask_user_question: {
    description: "Ask a guided setup question",
    consequential: false,
  },
  create_agent: { description: "Create a new agent", consequential: true },
  update_agent: { description: "Edit an existing agent", consequential: true },
  // Capability tools
  calculator: { description: "Evaluate arithmetic", consequential: false },
  fetch_url: { description: "Read a web page", consequential: false },
  web_search: { description: "Search the web", consequential: false },
  read_file: { description: "Read files in its workspace", consequential: false },
  write_file: { description: "Save files in its workspace", consequential: true },
};

export function toolInfo(name: string): ToolInfo {
  return TOOL_CATALOG[name] ?? { description: "", consequential: false };
}
