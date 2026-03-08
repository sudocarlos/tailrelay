const fs = require('fs');
const path = '/home/geru/.config/opencode/opencode.json';
const data = JSON.parse(fs.readFileSync(path, 'utf8'));

if (!data.mcpServers) {
  data.mcpServers = {};
}

data.mcpServers.playwright = {
  command: "npx",
  args: ["-y", "@playwright/mcp"]
};

data.mcpServers.context7 = {
  command: "npx",
  args: ["-y", "@upstash/context7-mcp"]
};

fs.writeFileSync(path, JSON.stringify(data, null, 2) + '\n', 'utf8');
console.log('Successfully updated opencode.json');
