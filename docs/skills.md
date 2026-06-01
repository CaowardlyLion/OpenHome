# Skills

Skills are Markdown instruction playbooks under `skills/`. They are not tools.

```md
---
name: grocery-list
description: Create grouped grocery lists.
allowedTools:
  - listFiles
  - readFile
  - writeFile
---

# Grocery List

Read existing list before editing it.
```

Catalog loading validates required metadata, duplicate names, and unknown tools.
The librarian selects once for the first execution unit. Later planned outcomes
reuse that skill unless the executor calls `request_skill_reselection`.
