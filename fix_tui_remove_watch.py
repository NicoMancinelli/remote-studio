import re

with open("lib/tui.sh", "r") as f:
    content = f.read()

# Remove watch-service from the menu entries
content = re.sub(r'\s*"watch-service"\s*"Manage auto-session watch service" \\', '', content)
content = re.sub(r'\s*"watch-service"\s*"Auto-session Watch Service" \\', '', content)
content = re.sub(r'\s*"watch-service"\s*"Watch Service \(legacy\)" \\', '', content)

# Sometimes it's listed as just text. Let's find the exact string.
